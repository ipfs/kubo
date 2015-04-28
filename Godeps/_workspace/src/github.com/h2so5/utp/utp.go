package utp

import (
	"errors"
	"time"
)

const (
	version = 1

	stData  = 0
	stFin   = 1
	stState = 2
	stReset = 3
	stSyn   = 4

	stateClosed = iota
	stateClosing
	stateSynSent
	stateConnected
	stateFinSent

	extNone         = 0
	extSelectiveAck = 1

	headerSize       = 20
	mtu              = 3200
	mss              = mtu - headerSize
	windowSize       = 100
	packetBufferSize = 256
	readBufferSize   = 1048576
	maxRetry         = 3

	maxUdpPayload = 65507
	resetTimeout  = time.Second
)

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

var (
	errTimeout error = &timeoutError{}
	errClosing       = errors.New("use of closed network connection")
)
