package gc_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/bodgit/gc"
	"github.com/stretchr/testify/assert"
)

func TestNewWriter(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)

	wc, err := gc.NewWriter(buf, gc.FormatTime(0), gc.CardSize(gc.MemoryCard59), gc.Encoding(gc.EncodingANSI))
	if err != nil {
		t.Fatal(err)
	}

	if err := wc.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join("testdata", "blank.mcd"))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, b, buf.Bytes())
}

func TestCopy(t *testing.T) {
	t.Parallel()

	rc, err := gc.OpenReader(filepath.Join("testdata", "0251b_2020_04Apr_01_05-02-47.raw"))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	wc, err := gc.NewWriter(io.Discard, gc.FlashID(rc.FlashID), gc.CardSize(rc.CardSize), gc.Encoding(rc.Encoding))
	if err != nil {
		t.Fatal(err)
	}
	defer wc.Close()

	fr, err := rc.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer fr.Close()

	fw, err := wc.Create()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(fw, fr); err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, fw.Close())
}

func ExampleWriter() {
	buf := new(bytes.Buffer)

	w, err := gc.NewWriter(buf)
	if err != nil {
		panic(err)
	}

	if err := w.Close(); err != nil {
		panic(err)
	}

	fmt.Println(buf.Len())
	// Output: 524288
}
