package gc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	errDuplicateName = errors.New("duplicate name")
	errInvalidLength = errors.New("invalid length")
	errNameMismatch  = errors.New("name mismatch")
	errNoFreeSpace   = errors.New("no free space")
)

type fileWriter struct {
	name string
	buf  *bytes.Buffer
	w    *Writer
}

func (w *fileWriter) maxSize() int {
	// Maximum file size is the size of the card plus the size of the
	// directory entry, (i.e. .gci header), minus the reserved block size
	return w.w.mc.size() + binary.Size(entry{}) - reservedBlocks*blockSize
}

func (w *fileWriter) Write(p []byte) (int, error) {
	if len(p)+w.buf.Len() > w.maxSize() {
		// Would exceed the maximum size
		return 0, errInvalidLength
	}

	return w.buf.Write(p) //nolint:wrapcheck
}

func (w *fileWriter) Close() error {
	w.w.mu.Lock()
	defer w.w.mu.Unlock()

	mc := w.w.mc

	e := new(entry)
	if err := binary.Read(w.buf, binary.BigEndian, e); err != nil {
		return fmt.Errorf("unable to read header: %w", err)
	}

	if e.filename() != w.name {
		return errNameMismatch
	}

	if w.buf.Len() != int(e.FileLength)*blockSize {
		return errInvalidLength
	}

	if mc.count() == 127 || e.FileLength > mc.blockMap[mc.activeBlockMap()].FreeBlocks {
		return errNoFreeSpace
	}

	// Set e.FirstBlock to the correct location
	e.FirstBlock = mc.blockMap[mc.activeBlockMap()].LastAllocatedBlock + 1

	// Write out the blocks
	lastBlock := e.FirstBlock + e.FileLength - reservedBlocks
	for i := e.FirstBlock - reservedBlocks; i < lastBlock; i++ {
		_, _ = w.buf.Read(mc.blocks[i][:])

		if i+1 < lastBlock {
			mc.blockMap[mc.activeBlockMap()].Blocks[i] = i + reservedBlocks + 1
		} else {
			mc.blockMap[mc.activeBlockMap()].Blocks[i] = 0xffff
		}
	}

	mc.directory[mc.activeDirectory()].Entries[mc.count()] = *e

	mc.blockMap[mc.activeBlockMap()].LastAllocatedBlock += e.FileLength
	mc.blockMap[mc.activeBlockMap()].FreeBlocks -= e.FileLength

	return mc.checksum()
}

type Writer struct {
	mu sync.Mutex
	w  io.Writer
	mc *memoryCard
}

func (w *Writer) Create(name string) (io.WriteCloser, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, e := range w.mc.directory[w.mc.activeDirectory()].Entries {
		if e.isEmpty() {
			continue
		}

		if e.filename() == name {
			return nil, errDuplicateName
		}
	}

	return &fileWriter{name, new(bytes.Buffer), w}, nil
}

func (w *Writer) Close() error {
	b, err := w.mc.MarshalBinary()
	if err != nil {
		return err
	}

	if n, err := w.w.Write(b); err != nil || n != w.mc.size() {
		if err != nil {
			return err //nolint:wrapcheck
		}

		return errInvalidLength
	}

	return nil
}

func NewWriter(w io.Writer, capacity, encoding uint16) (*Writer, error) {
	mc, err := newMemoryCard(capacity, encoding)
	if err != nil {
		return nil, err
	}

	return &Writer{
		w:  w,
		mc: mc,
	}, nil
}
