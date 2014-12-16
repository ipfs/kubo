// Extensions for Protocol Buffers to create more go like structures.
//
// Copyright (c) 2013, Vastech SA (PTY) LTD. All rights reserved.
// http://code.google.com/p/gogoprotobuf/gogoproto
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package io

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/proto"
	"encoding/binary"
	"errors"
	"io"
)

var (
	errSmallBuffer = errors.New("Buffer Too Small")
	errLargeValue  = errors.New("Value is Larger than 64 bits")
)

func NewDelimitedWriter(w io.Writer) WriteCloser {
	return &varintWriter{w, make([]byte, 10), nil}
}

type varintWriter struct {
	w      io.Writer
	lenBuf []byte
	buffer []byte
}

func (this *varintWriter) WriteMsg(msg proto.Message) (err error) {
	var data []byte
	if m, ok := msg.(marshaler); ok {
		n := m.Size()
		if n >= len(this.buffer) {
			this.buffer = make([]byte, n)
		}
		_, err = m.MarshalTo(this.buffer)
		if err != nil {
			return err
		}
		data = this.buffer[:n]
	} else {
		data, err = proto.Marshal(msg)
		if err != nil {
			return err
		}
	}
	length := uint64(len(data))
	n := binary.PutUvarint(this.lenBuf, length)
	_, err = this.w.Write(this.lenBuf[:n])
	if err != nil {
		return err
	}
	_, err = this.w.Write(data)
	return err
}

func (this *varintWriter) Close() error {
	if closer, ok := this.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func NewDelimitedReader(r io.Reader, maxSize int) ReadCloser {
	return &varintReader{r, make([]byte, 10), nil, maxSize}
}

type varintReader struct {
	r       io.Reader
	lenBuf  []byte
	buf     []byte
	maxSize int
}

func (this *varintReader) ReadMsg(msg proto.Message) error {
	firstLen, err := this.r.Read(this.lenBuf)
	if err != nil {
		return err
	}
	length64, lenLen := binary.Uvarint(this.lenBuf)
	if lenLen <= 0 {
		if lenLen == 0 {
			return errSmallBuffer
		}
		return errLargeValue
	}
	msgLen := int(length64)
	if len(this.buf) < msgLen {
		this.buf = make([]byte, msgLen)
	}
	prefixN := copy(this.buf, this.lenBuf[lenLen:firstLen])
	if _, err := io.ReadFull(this.r, this.buf[prefixN:msgLen]); err != nil {
		return err
	}
	return proto.Unmarshal(this.buf[:msgLen], msg)
}

func (this *varintReader) Close() error {
	if closer, ok := this.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
