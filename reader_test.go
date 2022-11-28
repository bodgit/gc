package gc_test

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/bodgit/gc"
)

func TestFS(t *testing.T) {
	t.Parallel()

	rc, err := gc.OpenReader(filepath.Join("testdata", "0251b_2020_04Apr_01_05-02-47.raw"))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	if err := fstest.TestFS(rc, "fzc.dat", "gc4sword"); err != nil {
		t.Fatal(err)
	}
}
