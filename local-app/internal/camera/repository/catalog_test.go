package repository

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"kicau.local/edge/internal/apperror"
)

func TestCatalogResolvesOnlyLocalSources(t *testing.T) {
	inputs := []Input{{ID: "one", Source: "rtsp://operator:secret@192.0.2.1/a"}, {ID: "two", Source: "rtsps://192.0.2.1/b"}}
	c, e := New(inputs, "rtsp://127.0.0.1:8554/")
	if e != nil {
		t.Fatal(e)
	}
	inputs[0].ID = "mutated"
	for id, want := range map[string]string{"one": "rtsp://127.0.0.1:8554/one", "two": "rtsp://127.0.0.1:8554/two"} {
		got, e := c.HubRTSP(id)
		if e != nil || got != want {
			t.Fatalf("%s: %s %v", id, got, e)
		}
	}
	if _, e = c.HubRTSP("missing"); !errors.Is(e, apperror.ErrNotFound) {
		t.Fatal(e)
	}
	exposed := c.Inputs()
	exposed[0].Source = "changed"
	if c.Inputs()[0].Source == "changed" {
		t.Fatal("catalog leaked mutable inputs")
	}
	channels := c.Channels()
	channels[0].ID = "changed"
	if c.Channels()[0].ID != "one" {
		t.Fatal("catalog leaked mutable channels")
	}
}

func TestCatalogParseErrorsPreserveCauseWithoutLoggingCredentials(t *testing.T) {
	const malformed = "rtsp://operator:private%zz@192.0.2.1/feed"
	for _, upstream := range []bool{false, true} {
		inputs := []Input{{ID: "one", Source: "rtsp://192.0.2.1/feed"}}
		base := "rtsp://127.0.0.1:8554"
		if upstream {
			inputs[0].Source = malformed
		} else {
			base = malformed
		}
		_, err := New(inputs, base)
		var parseErr *url.Error
		if !errors.As(err, &parseErr) {
			t.Fatalf("parser cause not preserved: %v", err)
		}
		for _, message := range []string{err.Error(), fmt.Sprintf("%+v", err)} {
			for _, secret := range []string{"operator", "private", "192.0.2.1", malformed} {
				if strings.Contains(message, secret) {
					t.Fatal("URL credentials or source leaked in error")
				}
			}
		}
	}
}
