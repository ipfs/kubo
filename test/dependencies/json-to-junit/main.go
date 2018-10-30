package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"log"
	"os"
)

type testMsg struct {
	Time    string
	Action  string
	Package string
	Test    string
	Output  string
	Elapsed float64
}

type testsuites struct {
	XMLName    xml.Name `xml:"testsuites"`
	Name       string   `xml:"name,attr"`
	Testsuites []testsuite
}

type testsuite struct {
	XMLName   xml.Name `xml:"testsuite"`
	Package   string   `xml:"package,attr"`
	Errors    int      `xml:"errors,attr"`
	Failures  int      `xml:"failures,attr"`
	Tests     int      `xml:"tests,attr"`
	Time      float64  `xml:"time,attr"`
	Testcases []testcase
}

type testcase struct {
	XMLName   xml.Name `xml:"testcase"`
	Name      string   `xml:"name,attr"`
	Classname string   `xml:"classname,attr"`
	Time      float64  `xml:"time,attr"`
	Sout      string   `xml:"system-out"`
	Serr      string   `xml:"system-err"`
	Skipped   string   `xml:"skipped,omitempty"`
	Failure   string   `xml:"failure,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	tests := make(map[string]map[string]testcase)
	var fails int
	var packages []testsuite
	for scanner.Scan() {
		msg := testMsg{}
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			log.Fatal(err)
		}

		switch {
		case msg.Action == "run":
			if tests[msg.Package] == nil {
				tests[msg.Package] = make(map[string]testcase)
			}

			tests[msg.Package][msg.Test] = testcase{
				Name:      msg.Test,
				Classname: msg.Package,
			}
		case msg.Action == "output":
			if msg.Test == "" {
				continue
			}

			test := tests[msg.Package][msg.Test]
			test.Sout = test.Sout + msg.Output + "\n"
			tests[msg.Package][msg.Test] = test
		case msg.Action == "skip":
			fallthrough
		case msg.Action == "fail":
			fallthrough
		case msg.Action == "pass":
			if msg.Test != "" {
				test := tests[msg.Package][msg.Test]
				test.Time = msg.Elapsed

				if msg.Action == "skip" {
					test.Skipped = "skipped"
				}

				if msg.Action == "fail" {
					fails++
					test.Failure = "failed"
				}

				tests[msg.Package][msg.Test] = test
				continue
			}

			ts := testsuite{
				Package: msg.Package,
				Time:    msg.Elapsed,
			}

			for _, test := range tests[msg.Package] {
				ts.Testcases = append(ts.Testcases, test)
				ts.Tests++
				if test.Failure != "" {
					ts.Failures++
				}
			}
			packages = append(packages, ts)
		case msg.Action == "cont":
		case msg.Action == "pause":
			// ??
		default:
			log.Fatalf("unknown action %s", msg.Action)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	out := testsuites{
		Name:       "go test",
		Testsuites: packages,
	}

	output, err := xml.MarshalIndent(&out, "  ", "    ")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	os.Stdout.Write(output)
	os.Exit(fails)
}
