package corehttp

import (
	"fmt"
	"io"
)

// The HTTP server uses seek to determine the file size. Actually _seeking_ can
// be slow so we wrap the seeker in a _lazy_ seeker.
type lazySeeker struct {
	reader io.ReadSeeker

	size       int64
	offset     int64
	realOffset int64
}

func (s *lazySeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekEnd:
		return s.Seek(s.size+offset, io.SeekStart)
	case io.SeekCurrent:
		return s.Seek(s.offset+offset, io.SeekStart)
	case io.SeekStart:
		if offset < 0 {
			return s.offset, fmt.Errorf("invalid seek offset")
		}
		s.offset = offset
		return s.offset, nil
	default:
		return s.offset, fmt.Errorf("invalid whence: %d", whence)
	}
}

func (s *lazySeeker) Read(b []byte) (int, error) {
	// If we're past the end, EOF.
	if s.offset >= s.size {
		return 0, io.EOF
	}

	// actually seek
	for s.offset != s.realOffset {
		off, err := s.reader.Seek(s.offset, io.SeekStart)
		if err != nil {
			return 0, err
		}
		s.realOffset = off
	}
	off, err := s.reader.Read(b)
	s.realOffset += int64(off)
	s.offset += int64(off)
	return off, err
}

func (s *lazySeeker) Close() error {
	if closer, ok := s.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
