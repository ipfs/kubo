package testutils

func FloatTruncate(value float64, decimalPlaces int) float64 {
	pow := 1.0
	for i := 0; i < decimalPlaces; i++ {
		pow *= 10.0
	}
	return float64(int(value*pow)) / pow
}
