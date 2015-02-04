package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/davecheney/profile"
	"github.com/dustin/go-humanize"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/h2so5/utp"
)

type RandReader struct{}

func (r RandReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(rand.Int())
	}
	return len(p), nil
}

type ByteCounter struct {
	n     int64
	mutex sync.RWMutex
}

func (b *ByteCounter) Write(p []byte) (n int, err error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.n += int64(len(p))
	return len(p), nil
}

func (b *ByteCounter) Length() int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.n
}

var h = flag.Bool("h", false, "Human readable")

func main() {
	var l = flag.Int("c", 10485760, "Payload length (bytes)")
	var s = flag.Bool("s", false, "Stream mode(Low memory usage, but Slow)")
	flag.Parse()

	defer profile.Start(profile.CPUProfile).Stop()

	if *h {
		fmt.Printf("Payload: %s\n", humanize.IBytes(uint64(*l)))
	} else {
		fmt.Printf("Payload: %d\n", *l)
	}

	c2s := c2s(int64(*l), *s)
	n, p := humanize.ComputeSI(c2s)
	if *h {
		fmt.Printf("C2S: %f%sbps\n", n, p)
	} else {
		fmt.Printf("C2S: %f\n", c2s)
	}

	s2c := s2c(int64(*l), *s)
	n, p = humanize.ComputeSI(s2c)
	if *h {
		fmt.Printf("S2C: %f%sbps\n", n, p)
	} else {
		fmt.Printf("S2C: %f\n", s2c)
	}

	avg := (c2s + s2c) / 2.0
	n, p = humanize.ComputeSI(avg)

	if *h {
		fmt.Printf("AVG: %f%sbps\n", n, p)
	} else {
		fmt.Printf("AVG: %f\n", avg)
	}
}

func c2s(l int64, stream bool) float64 {
	ln, err := utp.Listen("utp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}

	raddr, err := utp.ResolveUTPAddr("utp", ln.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	c, err := utp.DialUTPTimeout("utp", nil, raddr, 1000*time.Millisecond)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if err != nil {
		log.Fatal(err)
	}

	s, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	ln.Close()

	rch := make(chan int)

	sendHash := md5.New()
	readHash := md5.New()
	counter := ByteCounter{}

	var bps float64
	if stream {
		go func() {
			defer c.Close()
			io.Copy(io.MultiWriter(c, sendHash, &counter), io.LimitReader(RandReader{}, l))
		}()

		go func() {
			io.Copy(readHash, s)
			close(rch)
		}()

		go func() {
			for {
				select {
				case <-time.After(time.Second):
					if *h {
						fmt.Printf("\r <--> %s    ", humanize.IBytes(uint64(counter.Length())))
					} else {
						fmt.Printf("\r <--> %d    ", counter.Length())
					}
				case <-rch:
					fmt.Printf("\r")
					return
				}
			}
		}()

		start := time.Now()
		<-rch
		bps = float64(l*8) / (float64(time.Now().Sub(start)) / float64(time.Second))

	} else {
		var sendBuf, readBuf bytes.Buffer
		io.Copy(io.MultiWriter(&sendBuf, sendHash), io.LimitReader(RandReader{}, l))

		go func() {
			defer c.Close()
			io.Copy(c, &sendBuf)
		}()

		go func() {
			io.Copy(&readBuf, s)
			rch <- 0
		}()

		start := time.Now()
		<-rch
		bps = float64(l*8) / (float64(time.Now().Sub(start)) / float64(time.Second))

		io.Copy(sendHash, &sendBuf)
		io.Copy(readHash, &readBuf)
	}

	if !bytes.Equal(sendHash.Sum(nil), readHash.Sum(nil)) {
		log.Fatal("Broken payload")
	}

	return bps
}

func s2c(l int64, stream bool) float64 {
	ln, err := utp.Listen("utp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}

	raddr, err := utp.ResolveUTPAddr("utp", ln.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	c, err := utp.DialUTPTimeout("utp", nil, raddr, 1000*time.Millisecond)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if err != nil {
		log.Fatal(err)
	}

	s, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	ln.Close()

	rch := make(chan int)

	sendHash := md5.New()
	readHash := md5.New()
	counter := ByteCounter{}

	var bps float64

	if stream {
		go func() {
			defer s.Close()
			io.Copy(io.MultiWriter(s, sendHash, &counter), io.LimitReader(RandReader{}, l))
		}()

		go func() {
			io.Copy(readHash, c)
			close(rch)
		}()

		go func() {
			for {
				select {
				case <-time.After(time.Second):
					if *h {
						fmt.Printf("\r <--> %s    ", humanize.IBytes(uint64(counter.Length())))
					} else {
						fmt.Printf("\r <--> %d    ", counter.Length())
					}
				case <-rch:
					fmt.Printf("\r")
					return
				}
			}
		}()

		start := time.Now()
		<-rch
		bps = float64(l*8) / (float64(time.Now().Sub(start)) / float64(time.Second))

	} else {
		var sendBuf, readBuf bytes.Buffer
		io.Copy(io.MultiWriter(&sendBuf, sendHash), io.LimitReader(RandReader{}, l))

		go func() {
			defer s.Close()
			io.Copy(s, &sendBuf)
		}()

		go func() {
			io.Copy(&readBuf, c)
			rch <- 0
		}()

		start := time.Now()
		<-rch
		bps = float64(l*8) / (float64(time.Now().Sub(start)) / float64(time.Second))

		io.Copy(sendHash, &sendBuf)
		io.Copy(readHash, &readBuf)
	}

	if !bytes.Equal(sendHash.Sum(nil), readHash.Sum(nil)) {
		log.Fatal("Broken payload")
	}

	return bps
}
