package ffmpeg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizerRejectsExistingNonMKVBeforeProbe(t *testing.T) {
	output := filepath.Join(t.TempDir(), "unsupported.avi")
	if e := os.WriteFile(output, []byte("unit metadata fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := (Finalizer{}).Finalize(context.Background(), nil, output); e == nil {
		t.Fatal("existing unsupported container accepted")
	}
}
