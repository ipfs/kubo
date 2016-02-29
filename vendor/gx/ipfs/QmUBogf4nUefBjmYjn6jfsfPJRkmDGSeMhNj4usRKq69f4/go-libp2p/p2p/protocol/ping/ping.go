package ping

import (
	"bytes"
	"errors"
	"io"
	"time"

	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	host "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/host"
	inet "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/net"
	peer "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/peer"
	logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"
)

var log = logging.Logger("ping")

const PingSize = 32

const ID = "/ipfs/ping"

type PingService struct {
	Host host.Host
}

func NewPingService(h host.Host) *PingService {
	ps := &PingService{h}
	h.SetStreamHandler(ID, ps.PingHandler)
	return ps
}

func (p *PingService) PingHandler(s inet.Stream) {
	buf := make([]byte, PingSize)

	for {
		_, err := io.ReadFull(s, buf)
		if err != nil {
			log.Debug(err)
			return
		}

		_, err = s.Write(buf)
		if err != nil {
			log.Debug(err)
			return
		}
	}
}

func (ps *PingService) Ping(ctx context.Context, p peer.ID) (<-chan time.Duration, error) {
	s, err := ps.Host.NewStream(ctx, ID, p)
	if err != nil {
		return nil, err
	}

	out := make(chan time.Duration)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				t, err := ping(s)
				if err != nil {
					log.Debugf("ping error: %s", err)
					return
				}

				select {
				case out <- t:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

func ping(s inet.Stream) (time.Duration, error) {
	buf := make([]byte, PingSize)
	u.NewTimeSeededRand().Read(buf)

	before := time.Now()
	_, err := s.Write(buf)
	if err != nil {
		return 0, err
	}

	rbuf := make([]byte, PingSize)
	_, err = io.ReadFull(s, rbuf)
	if err != nil {
		return 0, err
	}

	if !bytes.Equal(buf, rbuf) {
		return 0, errors.New("ping packet was incorrect!")
	}

	return time.Now().Sub(before), nil
}
