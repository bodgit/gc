package gc

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/bodgit/gc/internal/hash"
)

var errBadBlockMapChecksum = errors.New("bad block map checksum")

type blockMap struct {
	Checksum           [checksums][hash.Size]byte
	UpdateCounter      uint16
	FreeBlocks         uint16
	LastAllocatedBlock uint16
	Blocks             [0x0ffb]uint16
}

func (m *blockMap) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(binary.Size(m))

	_ = binary.Write(buf, binary.BigEndian, m)

	return buf.Bytes(), nil
}

func (m *blockMap) generateChecksums() ([]byte, []byte, error) {
	b, err := m.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	normal, inverted := checksum(b[checksums*hash.Size:])

	return normal, inverted, nil
}

func (m *blockMap) checksum() error {
	normal, inverted, err := m.generateChecksums()
	if err != nil {
		return err
	}

	copy(m.Checksum[checksumNormal][:], normal)
	copy(m.Checksum[checksumInverted][:], inverted)

	return nil
}

func (m *blockMap) isValid() error {
	normal, inverted, err := m.generateChecksums()
	if err != nil {
		return err
	}

	c1, c2 := m.Checksum[checksumNormal][:], m.Checksum[checksumInverted][:]

	if !bytes.Equal(c1, normal) || !bytes.Equal(c2, inverted) {
		return errBadBlockMapChecksum
	}

	return nil
}

func newBlockMap(updateCounter, freeBlocks uint16) blockMap {
	return blockMap{
		UpdateCounter:      updateCounter,
		FreeBlocks:         freeBlocks,
		LastAllocatedBlock: reservedBlocks - 1,
	}
}
