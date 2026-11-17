package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestS3UploadUsesStableKey(t *testing.T) {
	body := "synthetic finalized clip"
	received := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/clips/rounds/r1/lapak01/id.mkv" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		b, _ := io.ReadAll(r.Body)
		received <- string(b)
		w.Header().Set("ETag", `"test"`)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := Options{Endpoint: srv.URL, Region: "us-east-1", Bucket: "clips", AccessKey: "test", SecretKey: "test"}
	s, e := New(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "clip.mkv")
	if e = os.WriteFile(file, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = s.Put(context.Background(), "rounds/r1/lapak01/id.mkv", file); e != nil {
			t.Fatal(e)
		}
		if got := <-received; got != body {
			t.Fatalf("body %q", got)
		}
	}
}
