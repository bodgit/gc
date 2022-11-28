package gc_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"path/filepath"
	"testing"

	"github.com/bodgit/gc"
	"github.com/stretchr/testify/assert"
)

//nolint:cyclop,funlen
func TestWriter(t *testing.T) {
	t.Parallel()

	rc, err := gc.OpenReader(filepath.Join("testdata", "0251b_2020_04Apr_01_05-02-47.raw"))
	if err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)

	wc, err := gc.NewWriter(buf, rc.CardSize(), rc.Encoding())
	if err != nil {
		t.Fatal(err)
	}

	h := sha256.New()

	for _, file := range rc.File {
		fw, err := wc.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}

		fr, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}

		// Copy the header but don't hash it
		if _, err := io.CopyN(fw, fr, 64); err != nil {
			t.Fatal(err)
		}

		// Copy and hash the rest of the file
		if _, err := io.Copy(io.MultiWriter(fw, h), fr); err != nil {
			t.Fatal(err)
		}

		if err := fr.Close(); err != nil {
			t.Fatal(err)
		}

		if err := fw.Close(); err != nil {
			t.Fatal(err)
		}
	}

	if err := wc.Close(); err != nil {
		t.Fatal(err)
	}

	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}

	h1 := h.Sum(nil)

	h.Reset()

	r, err := gc.NewReader(buf)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range r.File {
		fr, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}

		if _, err := io.CopyN(io.Discard, fr, 64); err != nil {
			t.Fatal(err)
		}

		if _, err := io.Copy(h, fr); err != nil {
			t.Fatal(err)
		}

		if err := fr.Close(); err != nil {
			t.Fatal(err)
		}
	}

	assert.Equal(t, h1, h.Sum(nil))
}
