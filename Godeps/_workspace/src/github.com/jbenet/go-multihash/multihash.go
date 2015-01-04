package multihash

import (
	"encoding/hex"
	"errors"
	"fmt"

	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
)

// errors
var (
	ErrUnknownCode     = errors.New("unknown multihash code")
	ErrTooShort        = errors.New("multihash too short. must be > 3 bytes")
	ErrTooLong         = errors.New("multihash too long. must be < 129 bytes")
	ErrLenNotSupported = errors.New("multihash does not yet support digests longer than 127 bytes")
)

// ErrInconsistentLen is returned when a decoded multihash has an inconsistent length
type ErrInconsistentLen struct {
	dm *DecodedMultihash
}

func (e ErrInconsistentLen) Error() string {
	return fmt.Sprintf("multihash length inconsistent: %v", e.dm)
}

// constants
const (
	SHA1     = 0x11
	SHA2_256 = 0x12
	SHA2_512 = 0x13
	SHA3     = 0x14
	BLAKE2B  = 0x40
	BLAKE2S  = 0x41
)

// Names maps the name of a hash to the code
var Names = map[string]int{
	"sha1":     SHA1,
	"sha2-256": SHA2_256,
	"sha2-512": SHA2_512,
	"sha3":     SHA3,
	"blake2b":  BLAKE2B,
	"blake2s":  BLAKE2S,
}

// Codes maps a hash code to it's name
var Codes = map[int]string{
	SHA1:     "sha1",
	SHA2_256: "sha2-256",
	SHA2_512: "sha2-512",
	SHA3:     "sha3",
	BLAKE2B:  "blake2b",
	BLAKE2S:  "blake2s",
}

// DefaultLengths maps a hash code to it's default length
var DefaultLengths = map[int]int{
	SHA1:     20,
	SHA2_256: 32,
	SHA2_512: 64,
	SHA3:     64,
	BLAKE2B:  64,
	BLAKE2S:  32,
}

type DecodedMultihash struct {
	Code   int
	Name   string
	Length int
	Digest []byte
}

type Multihash []byte

func (m *Multihash) HexString() string {
	return hex.EncodeToString([]byte(*m))
}

func (m *Multihash) String() string {
	return m.HexString()
}

func FromHexString(s string) (Multihash, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return Multihash{}, err
	}

	return Cast(b)
}

func (m Multihash) B58String() string {
	return b58.Encode([]byte(m))
}

func FromB58String(s string) (m Multihash, err error) {
	// panic handler, in case we try accessing bytes incorrectly.
	defer func() {
		if e := recover(); e != nil {
			m = Multihash{}
			err = e.(error)
		}
	}()

	//b58 smells like it can panic...
	b := b58.Decode(s)
	return Cast(b)
}

func Cast(buf []byte) (Multihash, error) {
	dm, err := Decode(buf)
	if err != nil {
		return Multihash{}, err
	}

	if !ValidCode(dm.Code) {
		return Multihash{}, ErrUnknownCode
	}

	return Multihash(buf), nil
}

// Decode a hash from the given Multihash.
func Decode(buf []byte) (*DecodedMultihash, error) {

	if len(buf) < 3 {
		return nil, ErrTooShort
	}

	if len(buf) > 129 {
		return nil, ErrTooLong
	}

	dm := &DecodedMultihash{
		Code:   int(uint8(buf[0])),
		Name:   Codes[int(uint8(buf[0]))],
		Length: int(uint8(buf[1])),
		Digest: buf[2:],
	}

	if len(dm.Digest) != dm.Length {
		return nil, ErrInconsistentLen{dm}
	}

	return dm, nil
}

// Encode a hash digest along with the specified function code.
// Note: the length is derived from the length of the digest itself.
func Encode(buf []byte, code int) ([]byte, error) {

	if !ValidCode(code) {
		return nil, ErrUnknownCode
	}

	if len(buf) > 127 {
		return nil, ErrLenNotSupported
	}

	pre := make([]byte, 2)
	pre[0] = byte(uint8(code))
	pre[1] = byte(uint8(len(buf)))
	return append(pre, buf...), nil
}

func EncodeName(buf []byte, name string) ([]byte, error) {
	return Encode(buf, Names[name])
}

// ValidCode checks whether a multihash code is valid.
func ValidCode(code int) bool {
	if AppCode(code) {
		return true
	}

	if _, ok := Codes[code]; ok {
		return true
	}

	return false
}

// AppCode checks whether a multihash code is part of the App range.
func AppCode(code int) bool {
	return code >= 0 && code < 0x10
}
