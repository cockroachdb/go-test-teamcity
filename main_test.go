package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kr/pretty"
)

var (
	inDir  = "testdata/input"
	outDir = "testdata/output"

	timestampRegexp      = regexp.MustCompile(`timestamp='.*?'`)
	timestampReplacement = `timestamp='2017-01-02T04:05:06.789'`
)

func runTest(t *testing.T, json bool, vrbs bool, arts bool) {
	defer func(v bool) {
		verbose = v
	}(verbose)
	verbose = vrbs

	sep := ""
	if json {
		sep = ".json"
	}

	files, err := ioutil.ReadDir(inDir + sep)
	if err != nil {
		t.Error(err)
	}
	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			defer func(a string) {
				artifacts = a
			}(artifacts)
			{
				f, err := ioutil.TempFile("", "artifacts")
				if err != nil {
					t.Fatal(err)
				}
				artifacts = f.Name()
				f.Close()
			}
			goldenArtifacts := filepath.Join("testdata", fmt.Sprintf("%s-artifacts-verbose-%t", file.Name(), vrbs))

			inpath := filepath.Join(inDir+sep, file.Name())
			f, err := os.Open(inpath)
			if err != nil {
				t.Error(err)
			}
			in := bufio.NewReader(f)

			out := &bytes.Buffer{}
			if json {
				processJSON(in, out)
			} else {
				processReader(in, out)
			}
			actual := out.String()
			actual = timestampRegexp.ReplaceAllString(actual, timestampReplacement)

			outpath := filepath.Join(outDir+sep, file.Name())
			if vrbs {
				outpath += ".verbose"
			}
			t.Logf("input: %s", inpath)
			t.Logf("output: %s", outpath)
			expectedBytes, err := ioutil.ReadFile(outpath)
			if err != nil {
				t.Error(err)
			}
			expected := string(expectedBytes)
			expected = timestampRegexp.ReplaceAllString(expected, timestampReplacement)

			const rewriteOutput = true
			if rewriteOutput {
				_ = ioutil.WriteFile(outpath, []byte(actual), 0644)
			}

			if strings.Compare(expected, actual) != 0 {
				t.Errorf("expected:\n\n%s\nbut got:\n\n%s\n", expected, actual)
			}

			if arts {
				exp, _ := ioutil.ReadFile(goldenArtifacts)
				act, _ := ioutil.ReadFile(artifacts)

				if string(exp) != string(act) {
					t.Error("mismatch detected: ", pretty.Diff(exp, act))

					if err := ioutil.WriteFile(goldenArtifacts, act, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestProcessReader(t *testing.T) {
	for _, json := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%t", json), func(t *testing.T) {
			for _, vrbs := range []bool{false, true} {
				t.Run(fmt.Sprintf("verbose=%t", verbose), func(t *testing.T) {
					for _, arts := range []bool{false, true} {
						t.Run(fmt.Sprintf("artifacts=%t", verbose), func(t *testing.T) {
							runTest(t, json, vrbs, arts)
						})
					}
				})
			}
		})
	}
}
