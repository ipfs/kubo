package utp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

type header struct {
	typ, ver     int
	id           uint16
	t, diff, wnd uint32
	seq, ack     uint16
}

type extension struct {
	typ     int
	payload []byte
}

type packet struct {
	header  header
	ext     []extension
	payload []byte
}

type outgoingPacket struct {
	typ     int
	ext     []extension
	payload []byte
}

func (p *packet) MarshalBinary() ([]byte, error) {
	firstExt := ext_none
	if len(p.ext) > 0 {
		firstExt = p.ext[0].typ
	}
	buf := new(bytes.Buffer)
	var beforeExt = []interface{}{
		// | type  | ver   |
		uint8(((byte(p.header.typ) << 4) & 0xF0) | (byte(p.header.ver) & 0xF)),
		// | extension     |
		uint8(firstExt),
	}
	var afterExt = []interface{}{
		// | connection_id                 |
		uint16(p.header.id),
		// | timestamp_microseconds                                        |
		uint32(p.header.t),
		// | timestamp_difference_microseconds                             |
		uint32(p.header.diff),
		// | wnd_size                                                      |
		uint32(p.header.wnd),
		// | seq_nr                        |
		uint16(p.header.seq),
		// | ack_nr                        |
		uint16(p.header.ack),
	}

	for _, v := range beforeExt {
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			return nil, err
		}
	}

	if len(p.ext) > 0 {
		for i, e := range p.ext {
			next := ext_none
			if i < len(p.ext)-1 {
				next = p.ext[i+1].typ
			}
			var ext = []interface{}{
				// | extension     |
				uint8(next),
				// | len           |
				uint8(len(e.payload)),
			}
			for _, v := range ext {
				err := binary.Write(buf, binary.BigEndian, v)
				if err != nil {
					return nil, err
				}
			}
			_, err := buf.Write(e.payload)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, v := range afterExt {
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			return nil, err
		}
	}

	_, err := buf.Write(p.payload)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (p *packet) UnmarshalBinary(data []byte) error {
	p.ext = nil
	buf := bytes.NewReader(data)
	var tv, e uint8

	var beforeExt = []interface{}{
		// | type  | ver   |
		(*uint8)(&tv),
		// | extension     |
		(*uint8)(&e),
	}
	for _, v := range beforeExt {
		err := binary.Read(buf, binary.BigEndian, v)
		if err != nil {
			return err
		}
	}

	for e != ext_none {
		currentExt := int(e)
		var l uint8
		var ext = []interface{}{
			// | extension     |
			(*uint8)(&e),
			// | len           |
			(*uint8)(&l),
		}
		for _, v := range ext {
			err := binary.Read(buf, binary.BigEndian, v)
			if err != nil {
				return err
			}
		}
		payload := make([]byte, l)
		size, err := buf.Read(payload[:])
		if err != nil {
			return err
		}
		if size != len(payload) {
			return io.EOF
		}
		p.ext = append(p.ext, extension{typ: currentExt, payload: payload})
	}

	var afterExt = []interface{}{
		// | connection_id                 |
		(*uint16)(&p.header.id),
		// | timestamp_microseconds                                        |
		(*uint32)(&p.header.t),
		// | timestamp_difference_microseconds                             |
		(*uint32)(&p.header.diff),
		// | wnd_size                                                      |
		(*uint32)(&p.header.wnd),
		// | seq_nr                        |
		(*uint16)(&p.header.seq),
		// | ack_nr                        |
		(*uint16)(&p.header.ack),
	}
	for _, v := range afterExt {
		err := binary.Read(buf, binary.BigEndian, v)
		if err != nil {
			return err
		}
	}

	p.header.typ = int((tv >> 4) & 0xF)
	p.header.ver = int(tv & 0xF)

	l := buf.Len()
	if l > 0 {
		p.payload = p.payload[:l]
		_, err := buf.Read(p.payload[:])
		if err != nil {
			return err
		}
	}

	return nil
}

func (p packet) String() string {
	var s string = fmt.Sprintf("[%d ", p.header.id)
	switch p.header.typ {
	case st_data:
		s += "ST_DATA"
	case st_fin:
		s += "ST_FIN"
	case st_state:
		s += "ST_STATE"
	case st_reset:
		s += "ST_RESET"
	case st_syn:
		s += "ST_SYN"
	}
	s += fmt.Sprintf(" seq:%d ack:%d len:%d", p.header.seq, p.header.ack, len(p.payload))
	s += "]"
	return s
}

var globalPool packetPool

type packetPool struct {
	root  *packetPoolNode
	mutex sync.Mutex
}

type packetPoolNode struct {
	p    *packet
	next *packetPoolNode
}

func (o *packetPool) get() *packet {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	r := o.root
	if r != nil {
		o.root = o.root.next
		return r.p
	} else {
		return &packet{
			payload: make([]byte, 0, mss),
		}
	}
}

func (o *packetPool) put(p *packet) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.root = &packetPoolNode{
		p:    p,
		next: o.root,
	}
}
