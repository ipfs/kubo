package cli

func MustVal[V any](val V, err error) V {
	if err != nil {
		panic(err)
	}
	return val
}
