package usecase

import (
	"errors"
	"testing"

	"kicau.local/edge/internal/apperror"
	audioService "kicau.local/edge/internal/mediaengine/audio/service"
	cameraRepo "kicau.local/edge/internal/camera/repository"
)

func TestSubscriptionsSeparateChannelAudioAndGlobalMetrics(t *testing.T) {
	catalog, err := cameraRepo.New([]cameraRepo.Input{{ID: "one", Source: "rtsp://192.0.2.1/a"}, {ID: "two", Source: "rtsp://192.0.2.1/b"}}, "rtsp://127.0.0.1:8554")
	if err != nil {
		t.Fatal(err)
	}
	broadcast := audioService.NewBroadcaster()
	uc := New(Options{}, catalog, broadcast, nil, nil)
	if messages, cancel, err := uc.SubscribeAudio("missing"); !errors.Is(err, apperror.ErrNotFound) || messages != nil || cancel != nil {
		t.Fatal("unknown channel accepted")
	}
	uc.started = true
	if err := uc.Activate([]string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	defer uc.Deactivate()
	audio, cancelAudio, err := uc.SubscribeAudio("one")
	if err != nil {
		t.Fatal(err)
	}
	defer cancelAudio()
	metrics, cancelMetrics, err := uc.SubscribeMetrics()
	if err != nil {
		t.Fatal(err)
	}
	defer cancelMetrics()
	broadcast.Publish("audio:two", []byte("other channel"))
	select {
	case <-audio:
		t.Fatal("received audio for another channel")
	default:
	}
	broadcast.Publish("audio:one", []byte("pcm"))
	broadcast.Publish("metrics:", []byte("metric"))
	select {
	case got := <-audio:
		if string(got) != "pcm" {
			t.Fatal("audio payload changed")
		}
	default:
		t.Fatal("audio subscription missed its frame")
	}
	select {
	case got := <-metrics:
		if string(got) != "metric" {
			t.Fatal("metric payload changed")
		}
	default:
		t.Fatal("global metrics subscription missed its frame")
	}
	cancelAudio()
	broadcast.Publish("audio:one", []byte("late"))
	select {
	case <-audio:
		t.Fatal("canceled subscription received audio")
	default:
	}
}
