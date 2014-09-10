// Copyright (c) 2013-2014 Conformal Systems LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package base58

import (
	"bytes"
	"encoding/hex"
	"testing"
)

var stringTests = []struct {
	in  string
	out string
}{
	{"", ""},
	{" ", "Z"},
	{"-", "n"},
	{"0", "q"},
	{"1", "r"},
	{"-1", "4SU"},
	{"11", "4k8"},
	{"abc", "ZiCa"},
	{"1234598760", "3mJr7AoUXx2Wqd"},
	{"abcdefghijklmnopqrstuvwxyz", "3yxU3u1igY8WkgtjK92fbJQCd4BZiiT1v25f"},
	{"00000000000000000000000000000000000000000000000000000000000000", "3sN2THZeE9Eh9eYrwkvZqNstbHGvrxSAM7gXUXvyFQP8XvQLUqNCS27icwUeDT7ckHm4FUHM2mTVh1vbLmk7y"},
}

var invalidStringTests = []struct {
	in  string
	out string
}{
	{"0", ""},
	{"O", ""},
	{"I", ""},
	{"l", ""},
	{"3mJr0", ""},
	{"O3yxU", ""},
	{"3sNI", ""},
	{"4kl8", ""},
	{"0OIl", ""},
	{"!@#$%^&*()-_=+~`", ""},
}

var hexTests = []struct {
	in  string
	out string
}{
	{"61", "2g"},
	{"626262", "a3gV"},
	{"636363", "aPEr"},
	{"73696d706c792061206c6f6e6720737472696e67", "2cFupjhnEsSn59qHXstmK2ffpLv2"},
	{"00eb15231dfceb60925886b67d065299925915aeb172c06647", "1NS17iag9jJgTHD1VXjvLCEnZuQ3rJDE9L"},
	{"516b6fcd0f", "ABnLTmg"},
	{"bf4f89001e670274dd", "3SEo3LWLoPntC"},
	{"572e4794", "3EFU7m"},
	{"ecac89cad93923c02321", "EJDM8drfXA6uyA"},
	{"10c8511e", "Rt5zm"},
	{"00000000000000000000", "1111111111"},
}

func TestBase58(t *testing.T) {
	// Base58Encode tests
	for x, test := range stringTests {
		tmp := []byte(test.in)
		if res := Encode(tmp); res != test.out {
			t.Errorf("Base58Encode test #%d failed: got: %s want: %s",
				x, res, test.out)
			continue
		}
	}

	// Base58Decode tests
	for x, test := range hexTests {
		b, err := hex.DecodeString(test.in)
		if err != nil {
			t.Errorf("hex.DecodeString failed failed #%d: got: %s", x, test.in)
			continue
		}
		if res := Decode(test.out); bytes.Equal(res, b) != true {
			t.Errorf("Base58Decode test #%d failed: got: %q want: %q",
				x, res, test.in)
			continue
		}
	}

	// Base58Decode with invalid input
	for x, test := range invalidStringTests {
		if res := Decode(test.in); string(res) != test.out {
			t.Errorf("Base58Decode invalidString test #%d failed: got: %q want: %q",
				x, res, test.out)
			continue
		}
	}
}
