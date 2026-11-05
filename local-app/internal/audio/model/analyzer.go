package model

import (
	"encoding/binary"
	"math"
	"time"
)

func Analyze(id string, pcm []byte, sampleRate int, threshold float64, at time.Time) Metric {
	var peak, sum float64
	n := len(pcm) / 2
	for i := 0; i < n; i++ {
		x := math.Abs(float64(int16(binary.LittleEndian.Uint16(pcm[2*i:]))) / 32768)
		peak = math.Max(peak, x)
		sum += x * x
	}
	db := func(x float64) float64 {
		if x <= 0 {
			return -120
		}
		return math.Max(-120, 20*math.Log10(x))
	}
	rms := float64(0)
	if n > 0 {
		rms = math.Sqrt(sum / float64(n))
	}
	window := int64(n) * 1000 / int64(sampleRate)
	active := int64(0)
	if db(rms) >= threshold {
		active = window
	}
	return Metric{id, at, db(peak), db(rms), active, window}
}
