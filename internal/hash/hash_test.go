package hash_test

import (
	"testing"

	"github.com/bodgit/gc/internal/hash"
	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	t.Parallel()

	h := hash.New()

	assert.Equal(t, 2, h.Size())
	assert.Equal(t, 1, h.BlockSize())

	if _, err := h.Write([]byte("test")); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []byte{0xe7, 0xd9}, h.Sum(nil))

	h.Reset()

	assert.Equal(t, []byte{0x00, 0x00}, h.Sum(nil))
}

func TestInvertedHash(t *testing.T) {
	t.Parallel()

	h := hash.NewInverted()

	assert.Equal(t, 2, h.Size())
	assert.Equal(t, 1, h.BlockSize())

	if _, err := h.Write([]byte("test")); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []byte{0x18, 0x25}, h.Sum(nil))

	h.Reset()

	assert.Equal(t, []byte{0x00, 0x00}, h.Sum(nil))
}
