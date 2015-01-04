package multihash

import (
	"bytes"
	"testing"
)

type SumTestCase struct {
	code   int
	length int
	input  string
	hex    string
}

var sumTestCases = []SumTestCase{
	SumTestCase{SHA1, -1, "foo", "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"},
	SumTestCase{SHA1, 10, "foo", "110a0beec7b5ea3f0fdbc95d"},
	SumTestCase{SHA2_256, -1, "foo", "12202c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
	SumTestCase{SHA2_256, 16, "foo", "12102c26b46b68ffc68ff99b453c1d304134"},
	SumTestCase{SHA2_512, -1, "foo", "1340f7fbba6e0636f890e56fbbf3283e524c6fa3204ae298382d624741d0dc6638326e282c41be5e4254d8820772c5518a2c5a8c0c7f7eda19594a7eb539453e1ed7"},
	SumTestCase{SHA2_512, 32, "foo", "1320f7fbba6e0636f890e56fbbf3283e524c6fa3204ae298382d624741d0dc663832"},
}

func TestSum(t *testing.T) {

	for _, tc := range sumTestCases {

		m1, err := FromHexString(tc.hex)
		if err != nil {
			t.Error(err)
			continue
		}

		m2, err := Sum([]byte(tc.input), tc.code, tc.length)
		if err != nil {
			t.Error(tc.code, "sum failed.", err)
			continue
		}

		if !bytes.Equal(m1, m2) {
			t.Error(tc.code, "sum failed.", m1, m2)
		}

		s1 := m1.HexString()
		if s1 != tc.hex {
			t.Error("hex strings not the same")
		}

		s2 := m1.B58String()
		m3, err := FromB58String(s2)
		if err != nil {
			t.Error("failed to decode b58")
		} else if !bytes.Equal(m3, m1) {
			t.Error("b58 failing bytes")
		} else if s2 != m3.B58String() {
			t.Error("b58 failing string")
		}
	}
}

func BenchmarkSum(b *testing.B) {
	tc := sumTestCases[0]
	for i := 0; i < b.N; i++ {
		Sum([]byte(tc.input), tc.code, tc.length)
	}
}
