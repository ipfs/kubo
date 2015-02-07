package redis

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/fzzy/radix/redis/resp"
)

const (
	bufSize int = 4096
)

//* Common errors

var LoadingError error = errors.New("server is busy loading dataset in memory")
var PipelineQueueEmptyError error = errors.New("pipeline queue empty")

//* Client

// Client describes a Redis client.
type Client struct {
	// The connection the client talks to redis over. Don't touch this unless
	// you know what you're doing.
	Conn      net.Conn
	timeout   time.Duration
	reader    *bufio.Reader
	pending   []*request
	completed []*Reply
}

// request describes a client's request to the redis server
type request struct {
	cmd  string
	args []interface{}
}

// Dial connects to the given Redis server with the given timeout, which will be
// used as the read/write timeout when communicating with redis
func DialTimeout(network, addr string, timeout time.Duration) (*Client, error) {
	// establish a connection
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	c := new(Client)
	c.Conn = conn
	c.timeout = timeout
	c.reader = bufio.NewReaderSize(conn, bufSize)
	return c, nil
}

// Dial connects to the given Redis server.
func Dial(network, addr string) (*Client, error) {
	return DialTimeout(network, addr, time.Duration(0))
}

//* Public methods

// Close closes the connection.
func (c *Client) Close() error {
	return c.Conn.Close()
}

// Cmd calls the given Redis command.
func (c *Client) Cmd(cmd string, args ...interface{}) *Reply {
	err := c.writeRequest(&request{cmd, args})
	if err != nil {
		return &Reply{Type: ErrorReply, Err: err}
	}
	return c.ReadReply()
}

// Append adds the given call to the pipeline queue.
// Use GetReply() to read the reply.
func (c *Client) Append(cmd string, args ...interface{}) {
	c.pending = append(c.pending, &request{cmd, args})
}

// GetReply returns the reply for the next request in the pipeline queue.
// Error reply with PipelineQueueEmptyError is returned,
// if the pipeline queue is empty.
func (c *Client) GetReply() *Reply {
	if len(c.completed) > 0 {
		r := c.completed[0]
		c.completed = c.completed[1:]
		return r
	}
	c.completed = nil

	if len(c.pending) == 0 {
		return &Reply{Type: ErrorReply, Err: PipelineQueueEmptyError}
	}

	nreqs := len(c.pending)
	err := c.writeRequest(c.pending...)
	c.pending = nil
	if err != nil {
		return &Reply{Type: ErrorReply, Err: err}
	}
	r := c.ReadReply()
	c.completed = make([]*Reply, nreqs-1)
	for i := 0; i < nreqs-1; i++ {
		c.completed[i] = c.ReadReply()
	}

	return r
}

//* Private methods

func (c *Client) setReadTimeout() {
	if c.timeout != 0 {
		c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
	}
}

func (c *Client) setWriteTimeout() {
	if c.timeout != 0 {
		c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
	}
}

// This will read a redis reply off of the connection without sending anything
// first (useful after you've sent a SUSBSCRIBE command). This will block until
// a reply is received or the timeout is reached. On timeout an ErrorReply will
// be returned, you can check if it's a timeout like so:
//
//	r := conn.ReadReply()
//	if r.Err != nil {
//		if t, ok := r.Err.(*net.OpError); ok && t.Timeout() {
//			// Is timeout
//		} else {
//			// Not timeout
//		}
//	}
//
// Note: this is a more low-level function, you really shouldn't have to
// actually use it unless you're writing your own pub/sub code
func (c *Client) ReadReply() *Reply {
	c.setReadTimeout()
	return c.parse()
}

func (c *Client) writeRequest(requests ...*request) error {
	c.setWriteTimeout()
	for i := range requests {
		req := make([]interface{}, 0, len(requests[i].args)+1)
		req = append(req, requests[i].cmd)
		req = append(req, requests[i].args...)
		err := resp.WriteArbitraryAsFlattenedStrings(c.Conn, req)
		if err != nil {
			c.Close()
			return err
		}
	}
	return nil
}

func (c *Client) parse() *Reply {
	m, err := resp.ReadMessage(c.reader)
	if err != nil {
		if t, ok := err.(*net.OpError); !ok || !t.Timeout() {
			// close connection except timeout
			c.Close()
		}
		return &Reply{Type: ErrorReply, Err: err}
	}
	r, err := messageToReply(m)
	if err != nil {
		return &Reply{Type: ErrorReply, Err: err}
	}
	return r
}

// The error return parameter is for bubbling up parse errors and the like, if
// the error is sent by redis itself as an Err message type, then it will be
// sent back as an actual Reply (wrapped in a CmdError)
func messageToReply(m *resp.Message) (*Reply, error) {
	r := &Reply{}

	switch m.Type {
	case resp.Err:
		errMsg, err := m.Err()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(errMsg.Error(), "LOADING") {
			err = LoadingError
		} else {
			err = &CmdError{errMsg}
		}
		r.Type = ErrorReply
		r.Err = err

	case resp.SimpleStr:
		status, err := m.Bytes()
		if err != nil {
			return nil, err
		}
		r.Type = StatusReply
		r.buf = status

	case resp.Int:
		i, err := m.Int()
		if err != nil {
			return nil, err
		}
		r.Type = IntegerReply
		r.int = i

	case resp.BulkStr:
		b, err := m.Bytes()
		if err != nil {
			return nil, err
		}
		r.Type = BulkReply
		r.buf = b

	case resp.Nil:
		r.Type = NilReply

	case resp.Array:
		ms, err := m.Array()
		if err != nil {
			return nil, err
		}
		r.Type = MultiReply
		r.Elems = make([]*Reply, len(ms))
		for i := range ms {
			r.Elems[i], err = messageToReply(ms[i])
			if err != nil {
				return nil, err
			}
		}
	}

	return r, nil

}
