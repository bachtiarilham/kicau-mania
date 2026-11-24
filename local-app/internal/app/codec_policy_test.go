package app

import (
	"context"
	"errors"
	"fmt"
	"kicau.local/edge/config"
	cameraModel "kicau.local/edge/internal/camera/model"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	if os.Getenv("KICAU_UNIT_RESTORE_H264") == "1" {
		fmt.Print(`{"streams":[{"codec_type":"video","codec_name":"h264","nb_read_packets":"30"}]}`)
		os.Exit(0)
	}
}
func TestH264ConfigurationCannotStartRuntime(t *testing.T) {
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("KICAU_UNIT_RESTORE_H264", "1")
	dir := t.TempDir()
	_, e = NewContainer(context.Background(), config.Config{FFprobe: binary, DataDir: dir, MediaConfig: filepath.Join(dir, "mediamtx.json"), Channels: []config.Channel{{ID: "one", Source: "rtsp://192.0.2.1/nvr"}}})
	var failure *cameraModel.Failure
	if !errors.As(e, &failure) || failure.Code != "UNSUPPORTED_VIDEO_CODEC" {
		t.Fatal("stored/initial H264 source admitted", e)
	}
	if _, e = os.Stat(filepath.Join(dir, "mediamtx.json")); !os.IsNotExist(e) {
		t.Fatal("invalid source reached media startup")
	}
}
