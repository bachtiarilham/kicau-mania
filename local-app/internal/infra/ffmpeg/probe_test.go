package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	setup "kicau.local/edge/internal/camera/model"
	"os"
	"testing"
)

// Subprocess fixture supplies probe metadata only; it never generates media.
func init() {
	if payload := os.Getenv("KICAU_UNIT_PROBE_METADATA"); payload != "" {
		fmt.Print(payload)
		os.Exit(0)
	}
}
func TestProbeEnforcesHEVCWithoutEncoder(t *testing.T) {
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	for _, codec := range []string{"h264", "mpeg4", "hevc"} {
		t.Run(codec, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]any{"streams": []any{map[string]any{"codec_type": "video", "codec_name": codec, "width": 1920, "height": 1080, "r_frame_rate": "25/1", "has_b_frames": 2, "nb_read_packets": "50"}}})
			t.Setenv("KICAU_UNIT_PROBE_METADATA", string(payload))
			result, err := (Inspector{ProbeBinary: binary}).Probe(context.Background(), "rtsp://192.0.2.1/nvr")
			if codec == "hevc" {
				if err != nil || result.BFrames != 2 {
					t.Fatal("HEVC must pass without modifying frames", err)
				}
				return
			}
			var failure *setup.Failure
			if !errors.As(err, &failure) || failure.Code != "UNSUPPORTED_VIDEO_CODEC" {
				t.Fatal("unsupported source accepted", err)
			}
		})
	}
}
