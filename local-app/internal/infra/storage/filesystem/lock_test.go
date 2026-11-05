package filesystem

import (
	"path/filepath"
	"testing"
)

func TestSingleInstanceLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a, e := Lock(path)
	if e != nil {
		t.Fatal(e)
	}
	if b, e := Lock(path); e == nil {
		b.Close()
		a.Close()
		t.Fatal("second owner acquired lock")
	}
	a.Close()
	b, e := Lock(path)
	if e != nil {
		t.Fatal(e)
	}
	b.Close()
}
