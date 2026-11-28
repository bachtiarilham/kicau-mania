package config

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestArenaSettingsPreserveCredentialsAndUnrelatedConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	source := "rtsp://camera:demo%24secret@192.168.1.5:554/stream"
	raw := "# keep this comment\nENGINE_ACCESS_MODE=lan\nAPI_TOKEN=" + strings.Repeat("t", 40) +
		"\nHTTP_ADDR=127.0.0.1:8090\nRTSP_ADDR=127.0.0.1:8554\nOTHER_KEY='an untouched value'\nCHANNELS_JSON='[\n  {\"id\":\"gantangan03\",\"source\":\"" + source + "\"}\n]'\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	values, err := godotenv.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	t.Setenv("JUDGE_READ_CIDRS", "")
	base, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := ReadArena(base)
	if err != nil || len(settings.Channels) != 1 {
		t.Fatalf("read arena: %v", err)
	}
	if strings.Contains(settings.Channels[0].Source, "@") || !settings.Channels[0].HasCredentials {
		t.Fatal("GET exposed credentials or failed to detect them")
	}
	settings.ServerIP = "192.168.1.6"
	settings.JudgeIPs = []string{"192.168.1.58"}
	settings.Channels = append(settings.Channels, ArenaChannel{ID: ArenaCameraID, Source: "rtsp://192.168.1.30/stream"})
	if err := SaveArena(base, settings); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bytes), "# keep this comment") || !strings.Contains(string(bytes), "OTHER_KEY='an untouched value'") {
		t.Fatal("unrelated lines changed")
	}
	saved, err := godotenv.UnmarshalBytes(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if saved["JUDGE_READ_CIDRS"] != "192.168.1.58/32" || saved["RTSP_ADDR"] != "0.0.0.0:8554" || saved["HTTP_ADDR"] != "0.0.0.0:8090" {
		t.Fatal("incorrect LAN addresses after save")
	}
	retained, err := ArenaSource(base, settings.Channels[0])
	if err != nil {
		t.Fatal(err)
	}
	rtsp, err := url.Parse(retained)
	if err != nil {
		t.Fatal("saved source is invalid")
	}
	password, ok := rtsp.User.Password()
	if !ok || password != "demo$secret" || !strings.Contains(saved["CHANNELS_JSON"], ArenaCameraID) {
		t.Fatal("missing stored credential or overview camera")
	}
	if err := SaveArena(base, settings); !errors.Is(err, ErrArenaConflict) {
		t.Fatalf("stale edit should be rejected: %v", err)
	}
}
