package config

import (
	"encoding/json"
	"github.com/joho/godotenv"
	"path/filepath"
	"testing"
)

func TestExampleEnvironmentJSON(t *testing.T) {
	files, err := filepath.Glob("../.env*.example")
	if err != nil || len(files) == 0 {
		t.Fatal("environment templates missing", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			values, err := godotenv.Read(file)
			if err != nil {
				t.Fatal("invalid dotenv syntax")
			}
			for _, key := range []string{"CHANNELS_JSON", "RESTREAM_TARGETS_JSON"} {
				if raw, ok := values[key]; ok && !json.Valid([]byte(raw)) {
					t.Errorf("%s contains invalid JSON", key)
				}
			}
		})
	}
}

func TestPortableConfigurationLoads(t *testing.T) {
	values, err := godotenv.Read("../.env.portable.example")
	if err != nil {
		t.Fatal("portable dotenv unreadable")
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	cfg, err := Load("../.env.portable.example")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessMode != "lan" || len(cfg.Channels) == 0 {
		t.Fatal("portable channels missing")
	}
}
