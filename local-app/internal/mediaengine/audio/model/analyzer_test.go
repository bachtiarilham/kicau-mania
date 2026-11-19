package model

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func TestAudioEnergy(t *testing.T) {
	pcm := make([]byte, 6400)
	silent := Analyze("a", pcm, 16000, -35, time.Now())
	if silent.PeakDBFS != -120 || silent.RMSDBFS != -120 || silent.ActiveMS != 0 || silent.WindowMS != 200 {
		t.Fatalf("silence: %+v", silent)
	}
	for i := 0; i < len(pcm); i += 2 {
		binary.LittleEndian.PutUint16(pcm[i:], 16384)
	}
	m := Analyze("a", pcm, 16000, -35, time.Now())
	if math.Abs(m.RMSDBFS+6.0206) > 0.001 || m.ActiveMS != 200 {
		t.Fatalf("half scale: %+v", m)
	}
	binary.LittleEndian.PutUint16(pcm, 32768)
	m = Analyze("a", pcm, 16000, -35, time.Now())
	if m.PeakDBFS != 0 {
		t.Fatalf("negative full scale peak: %v", m.PeakDBFS)
	}

	t.Run("proximity distance rejection", func(t *testing.T) {
		// Distant noise (~5 meters away, e.g. amplitude 300 = ~-40.7 dBFS < threshold -35 dBFS)
		distantNoise := make([]byte, 6400)
		for i := 0; i < len(distantNoise); i += 2 {
			binary.LittleEndian.PutUint16(distantNoise[i:], 300)
		}
		distMetric := Analyze("lapak01", distantNoise, 16000, -35, time.Now())
		if distMetric.ActiveMS != 0 {
			t.Errorf("expected distant noise (< -35dB) to be inactive, got ActiveMS = %d", distMetric.ActiveMS)
		}

		// Close vocalization (< 1.5 meters away, e.g. amplitude 3000 = ~-20.7 dBFS >= threshold -35 dBFS)
		closeVocal := make([]byte, 6400)
		for i := 0; i < len(closeVocal); i += 2 {
			binary.LittleEndian.PutUint16(closeVocal[i:], 3000)
		}
		closeMetric := Analyze("lapak01", closeVocal, 16000, -35, time.Now())
		if closeMetric.ActiveMS != 200 {
			t.Errorf("expected close vocal (>= -35dB) to be active, got ActiveMS = %d", closeMetric.ActiveMS)
		}
	})
}
