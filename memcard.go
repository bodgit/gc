package gc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Supported memory card sizes.
const (
	MemoryCard59 = 4 << iota
	MemoryCard123
	MemoryCard251
	MemoryCard507
	MemoryCard1019
	MemoryCard2043
)

const (
	EncodingANSI = iota
	EncodingSJIS
)

const (
	checksumNormal = iota
	checksumInverted
	checksums
)

const (
	master = iota
	backup
	copies
)

const (
	blockSize      = 0x2000
	reservedBlocks = 5
)

var (
	errInvalidCapacity = errors.New("not a valid capacity")
	errTrailingBytes   = errors.New("trailing bytes")
)

type memoryCard struct {
	header    header
	directory [copies]directory
	blockMap  [copies]blockMap

	blocks [][blockSize]byte
}

func (mc *memoryCard) activeDirectory() int {
	if mc.directory[backup].UpdateCounter > mc.directory[master].UpdateCounter {
		return backup
	}

	return master
}

func (mc *memoryCard) activeBlockMap() int {
	if mc.blockMap[backup].UpdateCounter > mc.blockMap[master].UpdateCounter {
		return backup
	}

	return master
}

func (mc *memoryCard) size() int {
	return mc.header.size()
}

func (mc *memoryCard) count() int {
	count := 0

	for _, e := range mc.directory[mc.activeDirectory()].Entries {
		if e.isEmpty() {
			continue
		}
		count++
	}

	return count
}

func (mc *memoryCard) checksum() error {
	if err := mc.header.checksum(); err != nil {
		return err
	}

	for i := 0; i < copies; i++ {
		if err := mc.directory[i].checksum(); err != nil {
			return err
		}

		if err := mc.blockMap[i].checksum(); err != nil {
			return err
		}
	}

	return nil
}

func (mc *memoryCard) isValid() (bool, error) {
	ok, err := mc.header.isValid()
	if err != nil || !ok {
		return ok, err
	}

	for i := 0; i < copies; i++ {
		ok, err = mc.directory[i].isValid()
		if err != nil || !ok {
			return ok, err
		}

		ok, err = mc.blockMap[i].isValid()
		if err != nil || !ok {
			return ok, err
		}
	}

	return true, nil
}

func validateCardSize(capacity uint16) error {
	switch capacity {
	case MemoryCard59:
	case MemoryCard123:
	case MemoryCard251:
	case MemoryCard507:
	case MemoryCard1019:
	case MemoryCard2043:
		break
	default:
		return errInvalidCapacity
	}

	return nil
}

func (mc *memoryCard) unmarshalBinary(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &mc.header); err != nil {
		return fmt.Errorf("unable to read header: %w", err)
	}

	if err := validateCardSize(mc.header.CardSize); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &mc.directory); err != nil {
		return fmt.Errorf("unable to read directory: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &mc.blockMap); err != nil {
		return fmt.Errorf("unable to read block map: %w", err)
	}

	mc.blocks = make([][blockSize]byte, mc.header.blocks()-reservedBlocks)

	for i := range mc.blocks {
		if _, err := io.ReadFull(r, mc.blocks[i][:]); err != nil {
			return fmt.Errorf("unable to read block: %w", err)
		}
	}

	if n, _ := io.CopyN(io.Discard, r, 1); n > 0 {
		return errTrailingBytes
	}

	return nil
}

func (mc *memoryCard) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)

	if err := mc.unmarshalBinary(r); err != nil {
		return err
	}

	return nil
}

func (mc *memoryCard) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(mc.size())

	b, err := mc.header.MarshalBinary()
	if err != nil {
		return nil, err
	}

	_, _ = buf.Write(b)

	for i := 0; i < copies; i++ {
		b, err = mc.directory[i].MarshalBinary()
		if err != nil {
			return nil, err
		}

		_, _ = buf.Write(b)
	}

	for i := 0; i < copies; i++ {
		b, err = mc.blockMap[i].MarshalBinary()
		if err != nil {
			return nil, err
		}

		_, _ = buf.Write(b)
	}

	for i := range mc.blocks {
		_, _ = buf.Write(mc.blocks[i][:])
	}

	return buf.Bytes(), nil
}

func newMemoryCard(capacity, encoding uint16) (*memoryCard, error) {
	if err := validateCardSize(capacity); err != nil {
		return nil, err
	}

	header := header{
		CardSize: capacity,
		Encoding: encoding,
	}

	freeBlocks := header.blocks() - reservedBlocks

	mc := &memoryCard{
		header: header,
		directory: [copies]directory{
			{
				UpdateCounter: 1,
			},
			{
				UpdateCounter: 1,
			},
		},
		blockMap: [copies]blockMap{
			{
				UpdateCounter:      1,
				FreeBlocks:         uint16(freeBlocks),
				LastAllocatedBlock: reservedBlocks - 1,
			},
			{
				UpdateCounter:      1,
				FreeBlocks:         uint16(freeBlocks),
				LastAllocatedBlock: reservedBlocks - 1,
			},
		},
	}

	mc.blocks = make([][blockSize]byte, freeBlocks)
	for i := range mc.blocks {
		mc.blocks[i][0] = 0xff
		for j := 1; j < len(mc.blocks[i]); j *= 2 {
			copy(mc.blocks[i][j:], mc.blocks[i][:j])
		}
	}

	if err := mc.checksum(); err != nil {
		return nil, err
	}

	return mc, nil
}
