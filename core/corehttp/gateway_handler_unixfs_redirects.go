package corehttp

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type redirLine struct {
	matcher string
	to      string
	code    int
}

func (rdl redirLine) match(s string) (bool, error) {
	re, err := regexp.Compile(rdl.matcher)
	if err != nil {
		return false, fmt.Errorf("failed to compile %v: %v", rdl.matcher, err)
	}

	match := re.FindString(s)
	if match == "" {
		return false, nil
	}

	return true, nil
}

type redirs []redirLine

func newRedirs(f io.Reader) *redirs {
	ret := redirs{}
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		t := scanner.Text()
		if len(t) > 0 && t[0] == '#' {
			// comment, skip line
			continue
		}
		groups := strings.Fields(scanner.Text())
		if len(groups) >= 2 {
			matcher := groups[0]
			to := groups[1]
			// default to 302 (temporary redirect)
			code := http.StatusFound
			if len(groups) >= 3 {
				c, err := strconv.Atoi(groups[2])
				if err == nil {
					code = c
				}
			}
			ret = append(ret, redirLine{matcher, to, code})
		}
	}

	return &ret
}

// returns "" if no redir
func (r redirs) search(path string) (string, int) {
	for _, rdir := range r {
		m, err := rdir.match(path)
		if m && err == nil {
			return rdir.to, rdir.code
		}
	}

	return "", 0
}
