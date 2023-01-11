package gc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Based on http://hitmen.c02.at/files/yagcd/yagcd/chap12.html#sec12 and
// https://github.com/suloku/gcmm/blob/master/source/raw.h

// Supported memory card sizes.
const (
	MemoryCard59 uint16 = 4 << iota
	MemoryCard123
	MemoryCard251
	MemoryCard507
	MemoryCard1019
	MemoryCard2043
)

// Supported memory card encodings.
const (
	EncodingANSI uint16 = iota
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
	errInvalidBlockMapCounters  = errors.New("invalid block allocation map update counters")
	errInvalidCapacity          = errors.New("not a valid capacity")
	errInvalidDirectoryCounters = errors.New("invalid directory update counters")
	errInvalidEncoding          = errors.New("not a valid encoding")
	errTrailingBytes            = errors.New("trailing bytes")
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

func (mc *memoryCard) serialNumbers() (uint32, uint32) {
	return mc.header.serialNumbers()
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

func (mc *memoryCard) isValid() error {
	if err := mc.header.isValid(); err != nil {
		return err
	}

	for i := 0; i < copies; i++ {
		if err := mc.directory[i].isValid(); err != nil {
			return err
		}

		if err := mc.blockMap[i].isValid(); err != nil {
			return err
		}
	}

	diff := int(mc.directory[master].UpdateCounter) - int(mc.directory[backup].UpdateCounter)
	if diff != 1 && diff != -1 {
		return errInvalidDirectoryCounters
	}

	diff = int(mc.blockMap[master].UpdateCounter) - int(mc.blockMap[backup].UpdateCounter)
	if diff != 1 && diff != -1 {
		return errInvalidBlockMapCounters
	}

	return nil
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

func validateEncoding(encoding uint16) error {
	switch encoding {
	case EncodingANSI:
	case EncodingSJIS:
		break
	default:
		return errInvalidEncoding
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

	if err := validateEncoding(mc.header.Encoding); err != nil {
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

	return mc.isValid()
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

func newMemoryCard(flashID [12]byte, formatTime uint64, capacity, encoding uint16) (*memoryCard, error) {
	if err := validateCardSize(capacity); err != nil {
		return nil, err
	}

	if err := validateEncoding(encoding); err != nil {
		return nil, err
	}

	header := header{
		Serial:        computeSerial(flashID, formatTime),
		FormatTime:    formatTime,
		CardSize:      capacity,
		Encoding:      encoding,
		UpdateCounter: 0xffff, //nolint:gomnd
	}

	freeBlocks := header.blocks() - reservedBlocks

	mc := &memoryCard{
		header: header,
		directory: [copies]directory{
			newDirectory(1),
			newDirectory(0),
		},
		blockMap: [copies]blockMap{
			newBlockMap(1, uint16(freeBlocks)),
			newBlockMap(0, uint16(freeBlocks)),
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

// DetectMemoryCard works out if the io.ReaderAt r pointing to the data of size
// bytes looks sufficiently like a GameCube memory card image.
func DetectMemoryCard(r io.ReaderAt, size int64) (bool, error) {
	h := new(header)
	if size >= int64(binary.Size(h)) {
		sr := io.NewSectionReader(r, 0, int64(binary.Size(h)))

		if err := binary.Read(sr, binary.BigEndian, h); err != nil {
			return false, fmt.Errorf("unable to read header: %w", err)
		}

		if (h.Encoding == EncodingANSI || h.Encoding == EncodingSJIS) && size == int64(h.size()) {
			return true, nil
		}
	}

	return false, nil
}
