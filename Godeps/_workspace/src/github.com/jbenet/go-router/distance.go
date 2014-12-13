package router

// HammingDistance is a DistanceFunc that interprets Addresses as strings
// and uses their Hamming distance.
// Return -1 if the Addresses are not strings, or strings length don't match.
func HammingDistance(a1, a2 Address) int {
	s1, ok := a1.(string)
	if !ok {
		return -1
	}

	s2, ok := a2.(string)
	if !ok {
		return -1
	}

	// runes not code points
	r1 := []rune(s1)
	r2 := []rune(s2)

	// hamming distance requires equal length strings
	if len(r1) != len(r2) {
		return -1
	}

	d := 0
	for i := range r1 {
		if r1[i] != r2[i] {
			d++
		}
	}
	return d
}
