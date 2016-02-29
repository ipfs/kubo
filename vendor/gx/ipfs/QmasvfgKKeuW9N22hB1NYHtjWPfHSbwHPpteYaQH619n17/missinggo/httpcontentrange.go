package missinggo

import (
	"math"
	"regexp"
	"strconv"
)

type HTTPBytesContentRange struct {
	First, Last, Length int64
}

type HTTPBytesRange struct {
	First, Last int64
}

var (
	bytesContentRangeRegexp = regexp.MustCompile(`bytes[ =](\d+)-(\d+)/(\d+|\*)`)
	httpBytesRangeRegexp    = regexp.MustCompile(`bytes[ =](\d+)-(\d*)`)
)

func ParseHTTPBytesRange(s string) (ret HTTPBytesRange, ok bool) {
	ss := httpBytesRangeRegexp.FindStringSubmatch(s)
	if ss == nil {
		return
	}
	var err error
	ret.First, err = strconv.ParseInt(ss[1], 10, 64)
	if err != nil {
		return
	}
	if ss[2] == "" {
		ret.Last = math.MaxInt64
	} else {
		ret.Last, err = strconv.ParseInt(ss[2], 10, 64)
		if err != nil {
			return
		}
	}
	ok = true
	return
}

func ParseHTTPBytesContentRange(s string) (ret HTTPBytesContentRange, ok bool) {
	ss := bytesContentRangeRegexp.FindStringSubmatch(s)
	if ss == nil {
		return
	}
	var err error
	ret.First, err = strconv.ParseInt(ss[1], 10, 64)
	if err != nil {
		return
	}
	ret.Last, err = strconv.ParseInt(ss[2], 10, 64)
	if err != nil {
		return
	}
	if ss[3] == "*" {
		ret.Length = -1
	} else {
		ret.Length, err = strconv.ParseInt(ss[3], 10, 64)
		if err != nil {
			return
		}
	}
	ok = true
	return
}
