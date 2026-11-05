package subprocess

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFFmpegGracefulMP4(t *testing.T) {
	ff, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	file := filepath.Join(t.TempDir(), "clip.mp4.partial")
	p, e := Start(ctx, ff, []string{"-hide_banner", "-loglevel", "error", "-re", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=10", "-c:v", "libx264", "-movflags", "+frag_keyframe+empty_moov+default_base_moof", "-f", "mp4", file}, nil)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(700 * time.Millisecond)
	p.Stop()
	if e = p.Err(); e != nil {
		t.Fatal(e)
	}
	st, e := os.Stat(file)
	if e != nil || st.Size() < 1000 {
		t.Fatalf("MP4 not flushed: %v %v", st, e)
	}
	probe, e := exec.LookPath("ffprobe")
	if e != nil {
		t.Skip("ffprobe unavailable")
	}
	if out, e := exec.CommandContext(ctx, probe, "-v", "error", "-show_entries", "stream=codec_name", file).CombinedOutput(); e != nil {
		t.Fatalf("invalid MP4: %s %v", out, e)
	}
}
