package crypto

import "testing"

func BenchmarkSign1B(b *testing.B)      { RunBenchmarkSign(b, 1) }
func BenchmarkSign10B(b *testing.B)     { RunBenchmarkSign(b, 10) }
func BenchmarkSign100B(b *testing.B)    { RunBenchmarkSign(b, 100) }
func BenchmarkSign1000B(b *testing.B)   { RunBenchmarkSign(b, 1000) }
func BenchmarkSign10000B(b *testing.B)  { RunBenchmarkSign(b, 10000) }
func BenchmarkSign100000B(b *testing.B) { RunBenchmarkSign(b, 100000) }

func BenchmarkVerify1B(b *testing.B)      { RunBenchmarkVerify(b, 1) }
func BenchmarkVerify10B(b *testing.B)     { RunBenchmarkVerify(b, 10) }
func BenchmarkVerify100B(b *testing.B)    { RunBenchmarkVerify(b, 100) }
func BenchmarkVerify1000B(b *testing.B)   { RunBenchmarkVerify(b, 1000) }
func BenchmarkVerify10000B(b *testing.B)  { RunBenchmarkVerify(b, 10000) }
func BenchmarkVerify100000B(b *testing.B) { RunBenchmarkVerify(b, 100000) }

func RunBenchmarkSign(b *testing.B, numBytes int) {
	secret, _, err := GenerateKeyPair(RSA, 1024)
	if err != nil {
		b.Fatal(err)
	}
	someData := make([]byte, numBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := secret.Sign(someData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func RunBenchmarkVerify(b *testing.B, numBytes int) {
	secret, public, err := GenerateKeyPair(RSA, 1024)
	if err != nil {
		b.Fatal(err)
	}
	someData := make([]byte, numBytes)
	signature, err := secret.Sign(someData)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid, err := public.Verify(someData, signature)
		if err != nil {
			b.Fatal(err)
		}
		if !valid {
			b.Fatal("signature should be valid")
		}
	}
}
