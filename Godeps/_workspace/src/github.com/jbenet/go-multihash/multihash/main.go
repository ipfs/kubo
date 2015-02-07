package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

var usage = `usage: %s [options] [FILE]
Print or check multihash checksums.
With no FILE, or when FILE is -, read standard input.

Options:
`

// flags
var encodings = []string{"raw", "hex", "base58", "base64"}
var algorithms = []string{"sha1", "sha2-256", "sha2-512", "sha3"}
var encoding string
var algorithm string
var algorithmCode int
var length int
var checkRaw string
var checkMh mh.Multihash
var inputFilename string
var quiet bool

// joined names
var algoStr string
var encStr string

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}

	algoStr = "one of: " + strings.Join(algorithms, ", ")
	flag.StringVar(&algorithm, "algorithm", "sha2-256", algoStr)
	flag.StringVar(&algorithm, "a", "sha2-256", algoStr+" (shorthand)")

	encStr = "one of: " + strings.Join(encodings, ", ")
	flag.StringVar(&encoding, "encoding", "base58", encStr)
	flag.StringVar(&encoding, "e", "base58", encStr+" (shorthand)")

	checkStr := "check checksum matches"
	flag.StringVar(&checkRaw, "check", "", checkStr)
	flag.StringVar(&checkRaw, "c", "", checkStr+" (shorthand)")

	lengthStr := "checksums length in bits (truncate). -1 is default"
	flag.IntVar(&length, "length", -1, lengthStr)
	flag.IntVar(&length, "l", -1, lengthStr+" (shorthand)")

	quietStr := "quiet output (no newline on checksum, no error text)"
	flag.BoolVar(&quiet, "quiet", false, quietStr)
	flag.BoolVar(&quiet, "q", false, quietStr+" (shorthand)")
}

func strIn(a string, set []string) bool {
	for _, s := range set {
		if s == a {
			return true
		}
	}
	return false
}

func parseFlags() error {
	flag.Parse()

	if !strIn(algorithm, algorithms) {
		return fmt.Errorf("algorithm '%s' not %s", algorithm, algoStr)
	}
	var found bool
	algorithmCode, found = mh.Names[algorithm]
	if !found {
		return fmt.Errorf("algorithm '%s' not found (lib error, pls report).")
	}

	if !strIn(encoding, encodings) {
		return fmt.Errorf("encoding '%s' not %s", encoding, encStr)
	}

	if checkRaw != "" {
		var err error
		checkMh, err = Decode(encoding, checkRaw)
		if err != nil {
			return fmt.Errorf("fail to decode check '%s': %s", checkRaw, err)
		}
	}

	if length >= 0 {
		if length%8 != 0 {
			return fmt.Errorf("length must be multiple of 8")
		}
		length = length / 8

		if length > mh.DefaultLengths[algorithmCode] {
			length = mh.DefaultLengths[algorithmCode]
		}
	}
	return nil
}

func getInput() (io.ReadCloser, error) {
	args := flag.Args()

	switch {
	case len(args) < 1:
		inputFilename = "-"
		return os.Stdin, nil
	case args[0] == "-":
		inputFilename = "-"
		return os.Stdin, nil
	default:
		inputFilename = args[0]
		f, err := os.Open(args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to open '%s': %s", args[0], err)
		}
		return f, nil
	}
}

func check(h1 mh.Multihash, r io.Reader) error {
	h2, err := hash(r)
	if err != nil {
		return err
	}

	if !bytes.Equal(h1, h2) {
		if quiet {
			os.Exit(1)
		}
		return fmt.Errorf("computed checksum did not match (-q for no output)")
	}

	if !quiet {
		fmt.Println("OK checksums match (-q for no output)")
	}
	return nil
}

func hash(r io.Reader) (mh.Multihash, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return mh.Sum(b, algorithmCode, length)
}

func printHash(r io.Reader) error {
	h, err := hash(r)
	if err != nil {
		return err
	}

	s, err := Encode(encoding, h)
	if err != nil {
		return err
	}

	if quiet {
		fmt.Print(s)
	} else {
		fmt.Println(s)
	}
	return nil
}

func main() {
	checkErr := func(err error) {
		if err != nil {
			die("error: ", err)
		}
	}

	err := parseFlags()
	checkErr(err)

	inp, err := getInput()
	checkErr(err)

	if checkMh != nil {
		err = check(checkMh, inp)
		checkErr(err)
	} else {
		err = printHash(inp)
		checkErr(err)
	}
	inp.Close()
}

func die(v ...interface{}) {
	if !quiet {
		fmt.Fprint(os.Stderr, v...)
		fmt.Fprint(os.Stderr, "\n")
	}
	// flag.Usage()
	os.Exit(1)
}
