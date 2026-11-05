package s3

import (
	"context"
	"errors"
	"testing"
)

func TestUploaderDoesNotInitializeBeforeFirstUploadAndRetriesFailures(t *testing.T) {
	lazy := NewLazy(Options{Bucket: "clips"})
	calls := 0
	lazy.create = func(context.Context, Options) (*Uploader, error) { calls++; return nil, errors.New("cloud offline") }
	if calls != 0 {
		t.Fatal("constructor contacted cloud")
	}
	for i := 0; i < 2; i++ {
		if lazy.Put(context.Background(), "key", "file") == nil {
			t.Fatal("initialization failure hidden")
		}
	}
	if calls != 2 {
		t.Fatal("failed initialization not retried")
	}
}
