package gc

import (
	"bytes"
	"io"

	"github.com/bodgit/gc/internal/hash"
)

func checksum(b []byte) ([]byte, []byte) {
	h1 := hash.New()
	h2 := hash.NewInverted()

	_, _ = io.Copy(io.MultiWriter(h1, h2), bytes.NewReader(b))

	normal := h1.Sum(nil)
	if bytes.Equal(normal, []byte{0xff, 0xff}) {
		normal[0], normal[1] = 0x00, 0x00
	}

	inverted := h2.Sum(nil)
	if bytes.Equal(inverted, []byte{0xff, 0xff}) {
		inverted[0], inverted[1] = 0x00, 0x00
	}

	return normal, inverted
}
