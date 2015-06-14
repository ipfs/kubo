package crypt

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"io"

	ci "github.com/ipfs/go-ipfs/p2p/crypto"
)

func NewEncryptionStream(key []byte, iv []byte, r io.Reader) (io.Reader, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(blk, iv)

	return &cipher.StreamReader{
		R: r,
		S: stream,
	}, nil
}

func EncryptStreamWithKey(r io.Reader, pubk ci.PubKey) (io.Reader, error) {
	key := make([]byte, 16)
	rand.Read(key)

	iv := make([]byte, 16)
	rand.Read(iv)

	enckey, err := pubk.Encrypt(key)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	lbuf := make([]byte, 8)
	n := binary.PutUvarint(lbuf, uint64(len(enckey)))
	buf.Write(lbuf[:n])
	buf.Write(enckey)

	n = binary.PutUvarint(lbuf, uint64(len(iv)))
	buf.Write(lbuf[:n])
	buf.Write(iv)

	encr, err := NewEncryptionStream(key, iv, r)
	if err != nil {
		return nil, err
	}

	return io.MultiReader(buf, encr), nil
}

func NewDecryptionStream(key []byte, iv []byte, r io.Reader) (io.Reader, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCFBDecrypter(blk, iv)

	return cipher.StreamReader{
		R: r,
		S: stream,
	}, nil
}

func DecryptStreamWithKey(r io.Reader, k ci.PrivKey) (io.Reader, error) {
	// need to wrap with bufio for ByteReader interface
	br := bufio.NewReader(r)

	l, err := binary.ReadUvarint(br)
	if err != nil {
		return nil, err
	}

	enckey := make([]byte, l)
	_, err = io.ReadFull(br, enckey)
	if err != nil {
		return nil, err
	}

	l, err = binary.ReadUvarint(br)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, l)
	_, err = io.ReadFull(br, iv)
	if err != nil {
		return nil, err
	}

	key, err := k.Decrypt(enckey)
	if err != nil {
		return nil, err
	}

	return NewDecryptionStream(key, iv, br)
}
