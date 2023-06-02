package testutils

func SetUnion[T comparable](sets ...map[T]bool) map[T]bool {
	newM := map[T]bool{}
	for _, s := range sets {
		for k, v := range s {
			newM[k] = v
		}
	}
	return newM
}
