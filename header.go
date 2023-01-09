package gc

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/bodgit/gc/internal/hash"
)

var errBadHeaderChecksum = errors.New("bad header checksum")

const (
	headerReserved1Size   = 0x0004
	headerReserved2Offset = 0x0026
	headerReserved2Size   = 0x01d4
	headerReserved3Offset = 0x0200
	headerReserved3Size   = 0x1e00
)

//nolint:maligned
type header struct {
	Serial        [12]byte
	FormatTime    uint64 // Ticks since GameCube epoch
	CounterBias   uint32
	Lang          uint32
	Unknown       [headerReserved1Size]byte // Seems to be either 0 or 1 as a uint32
	DeviceID      uint16
	CardSize      uint16
	Encoding      uint16
	_             [headerReserved2Size]byte
	UpdateCounter uint16
	Checksum      [checksums][hash.Size]byte
	_             [headerReserved3Size]byte
}

func (h *header) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(binary.Size(h))

	_ = binary.Write(buf, binary.BigEndian, h)

	b := buf.Bytes()

	for i := 0; i < headerReserved2Size; i++ {
		b[headerReserved2Offset+i] = 0xff
	}

	for i := 0; i < headerReserved3Size; i++ {
		b[headerReserved3Offset+i] = 0xff
	}

	return b, nil
}

func (h *header) size() int {
	return int(h.CardSize) << 17 //nolint:gomnd
}

func (h *header) blocks() int {
	return int(h.CardSize) << 4 //nolint:gomnd
}

func (h *header) serialNumbers() (uint32, uint32) {
	buf := new(bytes.Buffer)
	buf.Grow(binary.Size(h))

	_ = binary.Write(buf, binary.BigEndian, h)

	serial := make([]uint32, 8) //nolint:gomnd
	_ = binary.Read(buf, binary.BigEndian, &serial)

	return serial[0] ^ serial[2] ^ serial[4] ^ serial[6], serial[1] ^ serial[3] ^ serial[5] ^ serial[7]
}

func (h *header) generateChecksums() ([]byte, []byte, error) {
	b, err := h.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	normal, inverted := checksum(b[:508])

	return normal, inverted, nil
}

func (h *header) checksum() error {
	normal, inverted, err := h.generateChecksums()
	if err != nil {
		return err
	}

	copy(h.Checksum[checksumNormal][:], normal)
	copy(h.Checksum[checksumInverted][:], inverted)

	return nil
}

func (h *header) isValid() error {
	normal, inverted, err := h.generateChecksums()
	if err != nil {
		return err
	}

	c1, c2 := h.Checksum[checksumNormal][:], h.Checksum[checksumInverted][:]

	if !bytes.Equal(c1, normal) || !bytes.Equal(c2, inverted) {
		return errBadHeaderChecksum
	}

	return nil
}
