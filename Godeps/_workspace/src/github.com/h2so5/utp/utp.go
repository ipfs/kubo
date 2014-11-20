package utp

import "time"

const (
	version = 1

	st_data  = 0
	st_fin   = 1
	st_state = 2
	st_reset = 3
	st_syn   = 4

	ext_none          = 0
	ext_selective_ack = 1

	header_size = 20
	mtu         = 3200
	mss         = mtu - header_size
	window_size = 100

	reset_timeout = time.Second
)

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
