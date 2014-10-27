package pipes

// Duplex is a simple duplex channel
type Duplex struct {
	In  chan []byte
	Out chan []byte
}

func NewDuplex(bufsize int) Duplex {
	return Duplex{
		In:  make(chan []byte, bufsize),
		Out: make(chan []byte, bufsize),
	}
}
