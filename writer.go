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
	errNoFreeSpace   = errors.New("no free space")
)

type fileWriter struct {
	buf *bytes.Buffer
	w   *Writer
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

	delete(w.w.fw, w)

	mc := w.w.mc

	e := new(entry)
	if err := binary.Read(w.buf, binary.BigEndian, e); err != nil {
		return fmt.Errorf("unable to read header: %w", err)
	}

	for _, x := range mc.directory[mc.activeDirectory()].Entries {
		if x.isEmpty() {
			continue
		}

		if x.filename() == e.filename() {
			return errDuplicateName
		}
	}

	if w.buf.Len() != int(e.FileLength)*blockSize {
		return errInvalidLength
	}

	if mc.count() == maxEntries || e.FileLength > mc.blockMap[mc.activeBlockMap()].FreeBlocks {
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

// A Writer is used for creating a new memory card image with files written to
// it.
type Writer struct {
	mu sync.Mutex
	w  io.Writer
	mc *memoryCard
	fw map[*fileWriter]struct{}
}

// Create returns an io.WriteCloser for writing a new file on the memory card.
// The file should consist of a 64 byte header followed by one or more 8 KiB
// blocks as indicated in the header.
func (w *Writer) Create() (io.WriteCloser, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.mc.count() == maxEntries || w.mc.blockMap[w.mc.activeBlockMap()].FreeBlocks == 0 {
		return nil, errNoFreeSpace
	}

	fw := &fileWriter{new(bytes.Buffer), w}
	w.fw[fw] = struct{}{}

	return fw, nil
}

// Close writes out the memory card to the underlying io.Writer. Any in-flight
// open memory card files are closed first.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Close any open files
	for fw := range w.fw {
		if err := fw.Close(); err != nil {
			return err
		}

		delete(w.fw, fw)
	}

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

// NewWriter returns a Writer targeting a new blank memory card with the
// provided capacity and encoding.
func NewWriter(w io.Writer, capacity, encoding uint16) (*Writer, error) {
	mc, err := newMemoryCard(capacity, encoding)
	if err != nil {
		return nil, err
	}

	return &Writer{
		w:  w,
		mc: mc,
		fw: make(map[*fileWriter]struct{}),
	}, nil
}
