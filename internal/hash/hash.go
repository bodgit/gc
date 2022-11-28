package hash

import "hash"

const (
	// BlockSize is the preferred block size.
	BlockSize = 1
	// Size is the size of the checksum in bytes.
	Size       = 2
	shiftWidth = 8
)

type digest struct {
	inverted bool
	crc      uint16
	pos      int
}

func (d *digest) BlockSize() int { return BlockSize }

func (d *digest) Reset() { d.crc, d.pos = 0, 1 }

func (d *digest) Size() int { return Size }

func (d *digest) Sum(data []byte) []byte {
	return append(data, byte(d.crc>>shiftWidth), byte(d.crc))
}

func (d *digest) Write(p []byte) (int, error) {
	for i := range p {
		t := p[i]
		if d.inverted {
			t ^= 0xff
		}

		d.crc += uint16(t) << (d.pos * shiftWidth)
		d.pos++
		d.pos %= 2
	}

	return len(p), nil
}

// New returns a new hash.Hash computing the checksum.
func New() hash.Hash {
	d := new(digest)
	d.Reset()

	return d
}

// NewInverted returns a new hash.Hash computing the inverted checksum.
func NewInverted() hash.Hash {
	d := new(digest)
	d.inverted = true
	d.Reset()

	return d
}
