package redis

import (
	"bufio"
	"bytes"
	. "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func dial(t *T) *Client {
	client, err := DialTimeout("tcp", "127.0.0.1:6379", 10*time.Second)
	assert.Nil(t, err)
	return client
}

func TestCmd(t *T) {
	c := dial(t)
	v, _ := c.Cmd("echo", "Hello, World!").Str()
	assert.Equal(t, "Hello, World!", v)

	// Test that a bad command properly returns a *CmdError
	err := c.Cmd("non-existant-cmd").Err
	assert.NotEqual(t, "", err.(*CmdError).Error())

	// Test that application level errors propagate correctly
	c.Cmd("sadd", "foo", "bar")
	_, err = c.Cmd("get", "foo").Str()
	assert.NotEqual(t, "", err.(*CmdError).Error())
}

func TestPipeline(t *T) {
	c := dial(t)
	c.Append("echo", "foo")
	c.Append("echo", "bar")
	c.Append("echo", "zot")

	v, _ := c.GetReply().Str()
	assert.Equal(t, "foo", v)

	v, _ = c.GetReply().Str()
	assert.Equal(t, "bar", v)

	v, _ = c.GetReply().Str()
	assert.Equal(t, "zot", v)

	r := c.GetReply()
	assert.Equal(t, ErrorReply, r.Type)
	assert.Equal(t, PipelineQueueEmptyError, r.Err)
}

func TestParse(t *T) {
	c := dial(t)

	parseString := func(b string) *Reply {
		c.reader = bufio.NewReader(bytes.NewBufferString(b))
		return c.parse()
	}

	// missing \n trailing
	r := parseString("foo")
	assert.Equal(t, ErrorReply, r.Type)
	assert.NotNil(t, r.Err)

	// error reply
	r = parseString("-ERR unknown command 'foobar'\r\n")
	assert.Equal(t, ErrorReply, r.Type)
	assert.Equal(t, "ERR unknown command 'foobar'", r.Err.Error())

	// LOADING error
	r = parseString("-LOADING Redis is loading the dataset in memory\r\n")
	assert.Equal(t, ErrorReply, r.Type)
	assert.Equal(t, LoadingError, r.Err)

	// status reply
	r = parseString("+OK\r\n")
	assert.Equal(t, StatusReply, r.Type)
	assert.Equal(t, []byte("OK"), r.buf)

	// integer reply
	r = parseString(":1337\r\n")
	assert.Equal(t, IntegerReply, r.Type)
	assert.Equal(t, int64(1337), r.int)

	// null bulk reply
	r = parseString("$-1\r\n")
	assert.Equal(t, NilReply, r.Type)

	// bulk reply
	r = parseString("$6\r\nfoobar\r\n")
	assert.Equal(t, BulkReply, r.Type)
	assert.Equal(t, []byte("foobar"), r.buf)

	// null multi bulk reply
	r = parseString("*-1\r\n")
	assert.Equal(t, NilReply, r.Type)

	// multi bulk reply
	r = parseString("*5\r\n:0\r\n:1\r\n:2\r\n:3\r\n$6\r\nfoobar\r\n")
	assert.Equal(t, MultiReply, r.Type)
	assert.Equal(t, 5, len(r.Elems))
	for i := 0; i < 4; i++ {
		assert.Equal(t, int64(i), r.Elems[i].int)
	}
	assert.Equal(t, []byte("foobar"), r.Elems[4].buf)
}
