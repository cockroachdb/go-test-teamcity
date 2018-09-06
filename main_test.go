package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

var (
	inDir  = "testdata/input/"
	outDir = "testdata/output/"
)

func TestProcessReader(t *testing.T) {
	origGetNow := getNow
	defer func() {
		getNow = origGetNow
	}()
	files, err := ioutil.ReadDir(inDir)
	if err != nil {
		t.Error(err)
	}
	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			var tick int
			getNow = func() string {
				tick++
				return fmt.Sprintf("faketime #%05d", tick)
			}
			inpath := inDir + file.Name()
			f, err := os.Open(inpath)
			if err != nil {
				t.Error(err)
			}
			in := bufio.NewReader(f)

			out := &bytes.Buffer{}
			processReader(in, out)
			actual := out.String()

			outpath := outDir + file.Name()
			t.Logf("input: %s", inpath)
			t.Logf("output: %s", outpath)
			expectedBytes, err := ioutil.ReadFile(outpath)
			if err != nil {
				t.Error(err)
			}

			// Flip this to true during development to rewrite the output files.
			if false {
				if err := ioutil.WriteFile(outpath, []byte(actual), 0644); err != nil {
					t.Fatal(err)
				}
			}

			expected := string(expectedBytes)
			if strings.Compare(expected, actual) != 0 {
				t.Errorf("expected:\n\n%s\nbut got:\n\n%s\n", expected, actual)
			}
		})
	}
}
