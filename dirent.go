package gc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/bodgit/gc/internal/hash"
	"github.com/bodgit/plumbing"
)

var errBadDirectoryChecksum = errors.New("bad directory checksum")

const (
	entryReserved1Offset = 0x06
	entryReserved2Offset = 0x3a
	entryReserved2Size   = 0x02
)

//nolint:gochecknoglobals
var epoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

type entry struct {
	GameCode        [4]byte
	MakerCode       [2]byte
	_               byte
	BannerFormat    byte
	Filename        [32]byte
	LastModified    uint32
	ImageDataOffset uint32
	IconGfxFormat   uint16
	AnimationSpeed  uint16
	Permissions     byte
	CopyCounter     byte
	FirstBlock      uint16
	FileLength      uint16
	_               [entryReserved2Size]byte
	CommentAddress  uint32
}

func (e *entry) isEmpty() bool {
	e1 := bytes.Equal(e.GameCode[:], []byte{0xff, 0xff, 0xff, 0xff})
	e2 := bytes.Equal(e.GameCode[:], []byte{0x00, 0x00, 0x00, 0x00})

	return e1 || e2
}

func (e *entry) gameCode() string {
	return string(e.GameCode[:])
}

func (e *entry) makerCode() string {
	return string(e.MakerCode[:])
}

func (e *entry) filename() string {
	return string(bytes.TrimRight(e.Filename[:], "\x00"))
}

func (e *entry) lastModified() time.Time {
	return epoch.Add(time.Second * time.Duration(e.LastModified))
}

func (e *entry) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(binary.Size(e))

	_ = binary.Write(buf, binary.BigEndian, e)

	b := buf.Bytes()

	b[entryReserved1Offset] = 0xff

	for i := 0; i < entryReserved2Size; i++ {
		b[entryReserved2Offset+i] = 0xff
	}

	return b, nil
}

const (
	maxEntries             = 127
	directoryReserved1Size = 0x003a
)

type directory struct {
	Entries       [maxEntries]entry
	_             [directoryReserved1Size]byte
	UpdateCounter uint16
	Checksum      [checksums][hash.Size]byte
}

func (d *directory) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(binary.Size(d))

	for _, e := range d.Entries {
		b, err := e.MarshalBinary()
		if err != nil {
			return nil, err
		}

		_, _ = buf.Write(b)
	}

	//nolint:gomnd
	_, _ = io.CopyN(buf, plumbing.FillReader(0xff), directoryReserved1Size)

	_ = binary.Write(buf, binary.BigEndian, d.UpdateCounter)
	_ = binary.Write(buf, binary.BigEndian, d.Checksum)

	return buf.Bytes(), nil
}

func (d *directory) generateChecksums() ([]byte, []byte, error) {
	b, err := d.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	// Checksum everything upto the checksum fields
	normal, inverted := checksum(b[:blockSize-checksums*hash.Size])

	return normal, inverted, nil
}

func (d *directory) checksum() error {
	normal, inverted, err := d.generateChecksums()
	if err != nil {
		return err
	}

	copy(d.Checksum[checksumNormal][:], normal)
	copy(d.Checksum[checksumInverted][:], inverted)

	return nil
}

func (d *directory) isValid() error {
	normal, inverted, err := d.generateChecksums()
	if err != nil {
		return err
	}

	c1, c2 := d.Checksum[checksumNormal][:], d.Checksum[checksumInverted][:]

	if !bytes.Equal(c1, normal) || !bytes.Equal(c2, inverted) {
		return errBadDirectoryChecksum
	}

	return nil
}
