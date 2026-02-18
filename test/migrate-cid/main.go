package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"strings"

	cid "github.com/ipfs/go-cid"
)

// Command to parse a file and replace CIDv0 with CIDv1 strings.
// Single argument: the path to the file where the in-place replace will happen.

// These are hardcoded in `cid.Decode()`.
const CID_V0_PREFIX = "Qm"
const CID_V0_LENGTH = 46

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Missing argument with file path to replace CIDs.")
	}
	replaceFile := os.Args[1]

	input, err := ioutil.ReadFile(replaceFile)
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(input))

	outputFile, err := os.OpenFile(replaceFile, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()

	cidsFound := make([]cid.Cid, 0)
	cidsReplaceCount := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Find and store any CIDv0 in the current line.
		cidsFound = []cid.Cid{}
		var advanceLine int
		for {
			if c := findCidV0(line); c != cid.Undef {
				cidsFound = append(cidsFound, c)
				advanceLine = CID_V0_LENGTH
			} else {
				advanceLine = len(CID_V0_PREFIX)
			}
			if len(line) >= advanceLine {
				line = line[advanceLine:]
			} else {
				break
			}
		}

		// Start again from the start to actually replace the CID strings.
		// (Find and replace decoupled for readability. Performance is not a
		//  concern here.)
		line = scanner.Text()
		for _, cidV0 := range cidsFound {
			cidV1 := cid.NewCidV1(cid.DagProtobuf, cidV0.Hash())
			line = strings.Replace(line, cidV0.String(), cidV1.String(), -1)
		}
		outputFile.WriteString(line + "\n")
		cidsReplaceCount += len(cidsFound) // ignoring CIDs repeated in same string (rare)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Found and replaced %d CIDv0 strings.", cidsReplaceCount)
}

// Find first CIDv0 in string.
func findCidV0(s string) cid.Cid {
	cidStart := strings.Index(s, CID_V0_PREFIX)
	if cidStart == -1 {
		return cid.Undef
	}

	cidEnd := cidStart + CID_V0_LENGTH
	if cidEnd > len(s) {
		return cid.Undef
	}

	c, err := cid.Decode(s[cidStart:cidEnd])
	if err != nil {
		return cid.Undef
	}

	// This shouldn't happen but just in case check that the version is actually 0.
	if c.Version() != 0 {
		return cid.Undef
	}

	return c
}
