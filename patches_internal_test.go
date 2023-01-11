package gc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func copyData(f *File, wc *Writer) ([]byte, error) {
	fr, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer fr.Close()

	fw, err := wc.Create()
	if err != nil {
		return nil, err
	}
	defer fw.Close()

	// Skip header in checksum calculation
	if _, err := io.CopyN(fw, fr, int64(binary.Size(entry{}))); err != nil {
		return nil, fmt.Errorf("unable to copy save header: %w", err)
	}

	h := sha256.New()

	if _, err := io.Copy(io.MultiWriter(fw, h), fr); err != nil {
		return nil, fmt.Errorf("unable to copy save data: %w", err)
	}

	if err := fw.Close(); err != nil {
		return nil, fmt.Errorf("unable to close: %w", err)
	}

	return h.Sum(nil), nil
}

//nolint:cyclop,funlen
func TestPatches(t *testing.T) {
	t.Parallel()

	rc, err := OpenReader(filepath.Join("testdata", "patches.raw"))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	buf := new(bytes.Buffer)

	// Time needs to be set and non-zero to generate a static non-zero
	// serial number
	wc, err := NewWriter(buf, FormatTime(1))
	if err != nil {
		t.Fatal(err)
	}

	hashes := make(map[string][]byte)

	for _, f := range rc.File {
		b, err := copyData(f, wc)
		if err != nil {
			t.Fatal(err)
		}

		hashes[f.Name] = b
	}

	if err := wc.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(buf)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range r.File {
		fr, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}

		if _, err := io.CopyN(io.Discard, fr, int64(binary.Size(entry{}))); err != nil {
			t.Fatal(err)
		}

		h := sha256.New()

		if _, err := io.Copy(h, fr); err != nil {
			t.Fatal(err)
		}

		b := h.Sum(nil)

		match, ok := hashes[f.Name]
		if !ok {
			t.Fatal("unexpected file: ", f.Name)
		} else if bytes.Equal(match, b) {
			delete(hashes, f.Name)
		}
	}

	files := make([]string, 0, len(hashes))

	for k := range hashes {
		files = append(files, k)
	}

	assert.ElementsMatch(t, []string{"f_zero.dat", "PSO_SYSTEM", "PSO3_SYSTEM"}, files)
}
