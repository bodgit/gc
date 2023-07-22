package gc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
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

//nolint:gochecknoglobals
var gamePatches = map[string]func(io.Reader, *memoryCard) (io.Reader, error){
	"f_zero.dat":  patchFZero,
	"PSO_SYSTEM":  patchPSO12,
	"PSO3_SYSTEM": patchPSO3,
}

//nolint:cyclop,funlen
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

	var (
		r   io.Reader = w.buf
		err error
	)

	patchFunc, ok := gamePatches[e.filename()]
	if ok {
		if r, err = patchFunc(w.buf, mc); err != nil {
			return err
		}
	}

	// Set e.FirstBlock to the correct location
	e.FirstBlock = mc.blockMap[mc.activeBlockMap()].LastAllocatedBlock + 1

	// Write out the blocks
	lastBlock := e.FirstBlock + e.FileLength - reservedBlocks
	for i := e.FirstBlock - reservedBlocks; i < lastBlock; i++ {
		_, _ = r.Read(mc.blocks[i][:])

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
	mu         sync.Mutex
	w          io.Writer
	mc         *memoryCard
	fw         map[*fileWriter]struct{}
	formatTime uint64
	flashID    [12]byte
	cardSize   uint16
	encoding   uint16
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

// Credit to libogc/gc/ogc/lwp_watchdog.h.
const (
	busClock   uint64 = 162000000
	timerClock        = busClock / 4000
)

func now() uint64 {
	// Number of seconds since 00:00:00, 1st January 2000, expressed as ticks
	return timerClock * 1000 * uint64(time.Now().UTC().Sub(epoch).Seconds())
}

// NewWriter returns a Writer targeting a new blank memory card which defaults
// to 59 block capacity, ANSI encoding and an all-zeroes Flash ID.
func NewWriter(w io.Writer, options ...func(*Writer) error) (*Writer, error) {
	nw := &Writer{
		w:          w,
		fw:         make(map[*fileWriter]struct{}),
		formatTime: now(),
		cardSize:   MemoryCard59,
	}

	if err := nw.setOption(options...); err != nil {
		return nil, err
	}

	mc, err := newMemoryCard(nw.flashID, nw.formatTime, nw.cardSize, nw.encoding)
	if err != nil {
		return nil, err
	}

	nw.mc = mc

	return nw, nil
}

func (w *Writer) setOption(options ...func(*Writer) error) error {
	for _, option := range options {
		if err := option(w); err != nil {
			return err
		}
	}

	return nil
}

// FlashID sets the 12 byte Flash ID. If the target is an official memory card
// then this must be set correctly.
func FlashID(flashID [12]byte) func(*Writer) error {
	return func(w *Writer) error {
		w.flashID = flashID

		return nil
	}
}

// FormatTime overrides the formatting time, which is expressed in seconds
// since 00:00:00, 1st January 2000. Setting it to 0 stops the serial number
// from being computed.
func FormatTime(formatTime uint64) func(*Writer) error {
	return func(w *Writer) error {
		w.formatTime = formatTime

		return nil
	}
}

// CardSize sets the memory card capacity.
func CardSize(cardSize uint16) func(*Writer) error {
	return func(w *Writer) error {
		w.cardSize = cardSize

		return nil
	}
}

// Encoding sets the memory card encoding.
func Encoding(encoding uint16) func(*Writer) error {
	return func(w *Writer) error {
		w.encoding = encoding

		return nil
	}
}
