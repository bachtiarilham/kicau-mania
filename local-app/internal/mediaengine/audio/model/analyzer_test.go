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
}
