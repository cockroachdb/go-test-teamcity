package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	teamCityTimestampFormat = "2006-01-02T15:04:05.000"
)

// A Test represents a Go test result.
type Test struct {
	// StringOutput is a mock test that when output simply prints the given string.
	// For such a Test, everything else is assumed zero.
	StringOutput string

	// An actual test.
	Start, End string
	Name       string
	Output     string
	Details    []string
	Duration   time.Duration
	Status     string
	Race       bool
	Suite      bool

	Pkg string
}

var (
	input  = os.Stdin
	output = os.Stdout

	additionalTestName = ""

	run    = regexp.MustCompile("^=== RUN\\s+([a-zA-Z_]\\S*)")
	end    = regexp.MustCompile("^(\\s*)--- (PASS|SKIP|FAIL):\\s+([a-zA-Z_]\\S*) \\((-?[\\.\\ds]+)\\)")
	ending = regexp.MustCompile(`^PASS|FAIL|exit status|Found \d+ data race|coverage:`)
	pkg    = regexp.MustCompile(`^(?:ok|FAIL)\s+([a-z][^\s]*)`)
	race   = regexp.MustCompile("^WARNING: DATA RACE")
)

func init() {
	flag.StringVar(&additionalTestName, "name", "", "Add prefix to test name")
}

func escapeLines(lines []string) string {
	return escape(strings.Join(lines, "\n"))
}

func escape(s string) string {
	s = strings.Replace(s, "|", "||", -1)
	s = strings.Replace(s, "\n", "|n", -1)
	s = strings.Replace(s, "\r", "|n", -1)
	s = strings.Replace(s, "'", "|'", -1)
	s = strings.Replace(s, "]", "|]", -1)
	s = strings.Replace(s, "[", "|[", -1)
	return s
}

var getNow = func() string {
	return time.Now().Format(teamCityTimestampFormat)
}

type printer interface {
	WriteTo(pkg string)
}

func outputTest(w io.Writer, test *Test, pkg string) {
	if test.Pkg != "" {
		pkg = test.Pkg
	}
	if test.StringOutput != "" {
		fmt.Fprint(w, test.StringOutput)
		return
	}
	testName := escape(additionalTestName + test.Name)
	fmt.Fprintf(w, "##teamcity[testStarted timestamp='%s' pkg='%s' name='%s' captureStandardOutput='true']\n", test.Start, pkg, testName)
	fmt.Fprint(w, test.Output)
	if test.Status == "SKIP" {
		fmt.Fprintf(w, "##teamcity[testIgnored timestamp='%s' name='%s']\n", test.End, testName)
	} else {
		if test.Race {
			fmt.Fprintf(w, "##teamcity[testFailed timestamp='%s' name='%s' message='Race detected!' details='%s']\n",
				test.End, testName, escapeLines(test.Details))
		} else {
			switch test.Status {
			case "FAIL":
				fmt.Fprintf(w, "##teamcity[testFailed timestamp='%s' name='%s' details='%s']\n",
					test.End, testName, escapeLines(test.Details))
			case "PASS":
				// ignore
			default:
				fmt.Fprintf(w, "##teamcity[testFailed timestamp='%s' name='%s' message='Test ended in panic.' details='%s']\n",
					test.End, testName, escapeLines(test.Details))
			}
		}
		fmt.Fprintf(w, "##teamcity[testFinished timestamp='%s' name='%s' duration='%d']\n",
			test.End, testName, test.Duration/time.Millisecond)
	}
}

func startSuite(name string) *Test {
	return &Test{StringOutput: fmt.Sprintf("##teamcity[testSuiteStarted name='%s']\n", escape(name))}
}

func finishSuite(name string) *Test {
	return &Test{StringOutput: fmt.Sprintf("##teamcity[testSuiteFinished name='%s']\n", escape(name))}
}

func suite(name string) string {
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		return name[:idx]
	}
	return ""
}

func processReader(r *bufio.Reader, w io.Writer) {
	var staged []*Test // output when pkg finishes

	tests := map[string]*Test{}
	suites := []string{}
	var test *Test
	newTest := func(name string) *Test {
		t := &Test{
			Name:  name,
			Start: getNow(),
		}
		tests[t.Name] = t
		for n := suite(name); n != ""; n = suite(n) {
			if p := tests[n]; p != nil {
				p.Suite = true
			}
		}
		return t
	}
	var final string
	prefix := "\t"
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}

		runOut := run.FindStringSubmatch(line)
		endOut := end.FindStringSubmatch(line)
		pkgOut := pkg.FindStringSubmatch(line)
		endingOut := ending.FindStringSubmatch(line)

		if test != nil && test.Status != "" && (runOut != nil || endOut != nil || pkgOut != nil) {
			for j := len(suites) - 1; j >= 0; j-- {
				if !strings.HasPrefix(test.Name, suites[j]) {
					suites = suites[:j]
				}
			}
			if test.Suite {
				staged = append(staged, startSuite(test.Name))
				suites = append(suites, test.Name)
			}
			test.End = getNow()
			staged = append(staged, test)
			delete(tests, test.Name)
			test = nil
		}

		if runOut != nil {
			test = newTest(runOut[1])
		} else if endOut != nil {
			test = tests[endOut[3]]
			if test == nil {
				test = newTest(endOut[3])
			}
			prefix = endOut[1] + "\t"
			test.Status = endOut[2]
			test.Duration, _ = time.ParseDuration(endOut[4])
		} else if pkgOut != nil {
			for _, test := range staged {
				outputTest(w, test, pkgOut[1])
			}
			for _, test := range tests {
				// These tests haven't officially terminated yet (panic?), so
				// we can't output them yet. Instead, store the package in them
				// (which will override the package supplied to outputTest).
				test.Pkg = pkgOut[1]
			}
			staged = nil
			final += line
		} else if endingOut != nil {
			final += line
		} else if test != nil && race.MatchString(line) {
			test.Race = true
		} else if test != nil && test.Status != "" && strings.HasPrefix(line, prefix) {
			line = line[:len(line)-1]
			line = strings.TrimPrefix(line, prefix)
			test.Details = append(test.Details, line)
		} else if test != nil {
			test.Output += line
		} else {
			staged = append(staged, &Test{StringOutput: line})
		}
	}
	for _, test := range staged {
		outputTest(w, test, "unknown")
	}
	if test != nil {
		test.End = getNow()
		outputTest(w, test, "unknown")
		delete(tests, test.Name)
	}
	for j := len(suites) - 1; j >= 0; j-- {
		outputTest(w, finishSuite(suites[j]), "irrelevant")
	}
	for _, test := range tests {
		test.End = getNow()
		outputTest(w, test, "unknown")
	}

	fmt.Fprint(w, final)
}

func main() {
	flag.Parse()

	if len(additionalTestName) > 0 {
		additionalTestName += " "
	}

	reader := bufio.NewReader(input)

	processReader(reader, output)
}
