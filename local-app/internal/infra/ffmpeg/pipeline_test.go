package ffmpeg

import (
	"strings"
	"testing"
)

func TestAudioFilterString(t *testing.T) {
	t.Run("disabled filter returns empty string", func(t *testing.T) {
		p := Pipeline{
			AudioFilter: AudioFilterOptions{Enabled: false},
		}
		if got := p.AudioFilterString(); got != "" {
			t.Errorf("expected empty string when disabled, got %q", got)
		}
	})

	t.Run("enabled filter uses custom values", func(t *testing.T) {
		p := Pipeline{
			AudioFilter: AudioFilterOptions{
				Enabled:       true,
				HighpassFreq:  120,
				GateThreshold: -38.5,
				GateRatio:     5.0,
				GateAttack:    3.0,
				GateRelease:   200.0,
			},
		}
		expected := "highpass=f=120,agate=threshold=-38.5dB:ratio=5.0:attack=3.0:release=200.0"
		if got := p.AudioFilterString(); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("enabled filter uses default fallback for zero values", func(t *testing.T) {
		p := Pipeline{
			AudioFilter: AudioFilterOptions{
				Enabled: true,
			},
		}
		got := p.AudioFilterString()
		if !strings.Contains(got, "highpass=f=100") {
			t.Errorf("expected default highpass=100 in %q", got)
		}
		if !strings.Contains(got, "threshold=-55.0dB") {
			t.Errorf("expected default threshold=-35.0dB in %q", got)
		}
		if !strings.Contains(got, "ratio=4.0") {
			t.Errorf("expected default ratio=4.0 in %q", got)
		}
	})
}
