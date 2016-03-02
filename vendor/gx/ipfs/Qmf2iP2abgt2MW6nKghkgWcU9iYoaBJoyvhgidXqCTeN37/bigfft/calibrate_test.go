// Usage: go test -run=TestCalibrate -calibrate

package bigfft

import (
	"flag"
	"fmt"
	"testing"
	"time"
)

var calibrate = flag.Bool("calibrate", false, "run calibration test")

// measureMul benchmarks math/big versus FFT for a given input size
// (in bits).
func measureMul(th int) (tBig, tFFT time.Duration) {
	bigLoad := func(b *testing.B) { benchmarkMulBig(b, th, th) }
	fftLoad := func(b *testing.B) { benchmarkMulFFT(b, th, th) }

	res1 := testing.Benchmark(bigLoad)
	res2 := testing.Benchmark(fftLoad)
	tBig = time.Duration(res1.NsPerOp())
	tFFT = time.Duration(res2.NsPerOp())
	return
}

func TestCalibrateThreshold(t *testing.T) {
	if !*calibrate {
		t.Log("not calibrating, use -calibrate to do so.")
		return
	}

	lower := int(1e3)   // math/big is faster at this size.
	upper := int(300e3) // FFT is faster at this size.

	big, fft := measureMul(lower)
	lowerX := float64(big) / float64(fft)
	fmt.Printf("speedup at size %d: %.2f\n", lower, lowerX)
	big, fft = measureMul(upper)
	upperX := float64(big) / float64(fft)
	fmt.Printf("speedup at size %d: %.2f\n", upper, upperX)
	for {
		mid := (lower + upper) / 2
		big, fft := measureMul(mid)
		X := float64(big) / float64(fft)
		fmt.Printf("speedup at size %d: %.2f\n", mid, X)
		switch {
		case X < 0.98:
			lower = mid
			lowerX = X
		case X > 1.02:
			upper = mid
			upperX = X
		default:
			fmt.Printf("speedup at size %d: %.2f\n", lower, lowerX)
			fmt.Printf("speedup at size %d: %.2f\n", upper, upperX)
			return
		}
	}
}

func measureFFTSize(w int, k uint) time.Duration {
	load := func(b *testing.B) {
		x := rndNat(w)
		y := rndNat(w)
		for i := 0; i < b.N; i++ {
			m := (w+w)>>k + 1
			xp := polyFromNat(x, k, m)
			yp := polyFromNat(y, k, m)
			rp := xp.Mul(&yp)
			_ = rp.Int()
		}
	}
	res := testing.Benchmark(load)
	return time.Duration(res.NsPerOp())
}

func TestCalibrateFFT(t *testing.T) {
	if !*calibrate {
		t.Log("not calibrating, use -calibrate to do so.")
		return
	}

K:
	for k := uint(3); k < 16; k++ {
		// Measure the speedup between k and k+1
		low := 10 << k
		hi := 500 << k
		low1, low2 := measureFFTSize(low, k), measureFFTSize(low, k+1)
		lowX := float64(low1) / float64(low2) // less than 1
		fmt.Printf("speedup of %d vs %d at size %d words: %.2f\n", k+1, k, low, lowX)
		hi1, hi2 := measureFFTSize(hi, k), measureFFTSize(hi, k+1)
		hiX := float64(hi1) / float64(hi2) // larger than 1
		fmt.Printf("speedup of %d vs %d at size %d words: %.2f\n", k+1, k, hi, hiX)
		for i := 0; i < 10; i++ {
			mid := (low + hi) / 2
			mid1, mid2 := measureFFTSize(mid, k), measureFFTSize(mid, k+1)
			midX := float64(mid1) / float64(mid2)
			fmt.Printf("speedup of %d vs %d at size %d words: %.2f\n", k+1, k, mid, midX)
			switch {
			case midX < 0.98:
				low = mid
				lowX = midX
			case midX > 1.02:
				hi = mid
				hiX = midX
			default:
				fmt.Printf("speedup of %d vs %d at size %d words: %.2f\n", k+1, k, low, lowX)
				fmt.Printf("speedup of %d vs %d at size %d words: %.2f\n", k+1, k, hi, hiX)
				continue K
			}
		}
	}
}
