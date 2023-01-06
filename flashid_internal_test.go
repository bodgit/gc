package gc

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractFlashID(t *testing.T) {
	t.Parallel()

	var flashID [serialLength]byte

	if _, err := rand.Read(flashID[:]); err != nil {
		t.Fatal(err)
	}

	t0 := now()

	serial := computeSerial(flashID, t0)

	assert.Equal(t, flashID, extractFlashID(serial, t0))
}
