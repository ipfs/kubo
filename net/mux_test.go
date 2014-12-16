package mux

import (
	"bytes"
	"testing"

	swarm "github.com/jbenet/go-ipfs/net/swarm2"
)

var testCases = map[string]string{
	"bitswap": "\u0007bitswap",
	"dht":     "\u0003dht",
	"ipfs":    "\u0004ipfs",
	"ipfsdksnafkasnfkdajfkdajfdsjadosiaaodjasofdias": ".ipfsdksnafkasnfkdajfkdajfdsjadosiaaodjasofdias",
}

func TestWrite(t *testing.T) {
	for k, v := range testCases {
		var buf bytes.Buffer
		Write(&buf, k)

		v2 := buf.Bytes()
		if !bytes.Equal(v2, []byte(v)) {
			t.Errorf("failed: %s - %v != %v", k, []byte(v), v2)
		}
	}
}

func TestHandler(t *testing.T) {

	outs := make(chan string, 10)

	h := func(n string) func(s *swarm.Stream) {
		return func(s *swarm.Stream) {
			outs <- n
		}
	}

	m := Mux{Handlers: map[string]StreamHandler{}}
	m.Default = h("default")
	m.Handlers["dht"] = h("bitswap")
	// m.Handlers["ipfs"] = h("bitswap") // default!
	m.Handlers["bitswap"] = h("bitswap")
	m.Handlers["ipfsdksnafkasnfkdajfkdajfdsjadosiaaodjasofdias"] = h("bitswap")

	for k, v := range testCases {
		var buf bytes.Buffer
		if _, err := buf.Write([]byte(v)); err != nil {
			t.Error(err)
			continue
		}

		name, _, err := m.NextHandler(&buf)
		if err != nil {
			t.Error(err)
			continue
		}

		if name != k {
			t.Errorf("name mismatch: %s != %s", k, name)
			continue
		}
	}

}
