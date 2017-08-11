package gc

import (
	"math/rand"
	"testing"
)

func BenchmarkMapInserts(b *testing.B) {
	b.N = 10e6
	keys := make([]string, b.N)
	buf := make([]byte, 64)
	for i := 0; i < b.N; i++ {
		_, err := rand.Read(buf)
		if err != nil {
			b.Fatal(err)
		}
		keys[i] = string(buf)
	}

	set := make(map[string]trielement)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		set[keys[i]] = trielement(1)
	}
}
func BenchmarkMapUpdate(b *testing.B) {
	b.N = 10e6
	keys := make([]string, b.N)
	buf := make([]byte, 64)
	for i := 0; i < b.N; i++ {
		_, err := rand.Read(buf)
		if err != nil {
			b.Fatal(err)
		}
		keys[i] = string(buf)
	}

	set := make(map[string]trielement)
	for i := 0; i < b.N; i++ {
		set[keys[i]] = trielement(1)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		set[keys[i]] = trielement(2)
	}
}

type testint uint8
type teststruc struct {
	u uint8
}

func BenchmarkMapUpdateUint8(b *testing.B) {
	b.N = 10e6
	keys := make([]string, b.N)
	buf := make([]byte, 64)
	for i := 0; i < b.N; i++ {
		_, err := rand.Read(buf)
		if err != nil {
			b.Fatal(err)
		}
		keys[i] = string(buf)
	}

	set := make(map[string]testint)
	for i := 0; i < b.N; i++ {
		set[keys[i]] = testint(i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		set[keys[i]] = testint(i)
	}
}

func BenchmarkMapUpdateStruct(b *testing.B) {
	b.N = 10e6
	keys := make([]string, b.N)
	buf := make([]byte, 64)
	for i := 0; i < b.N; i++ {
		_, err := rand.Read(buf)
		if err != nil {
			b.Fatal(err)
		}
		keys[i] = string(buf)
	}

	set := make(map[string]teststruc)
	for i := 0; i < b.N; i++ {
		set[keys[i]] = teststruc{uint8(i)}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		set[keys[i]] = teststruc{uint8(i)}
	}
}
