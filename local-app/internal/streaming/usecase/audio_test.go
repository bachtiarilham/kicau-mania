package usecase

import (
	"errors"
	"testing"

	"kicau.local/edge/internal/apperror"
	cameraRepo "kicau.local/edge/internal/camera/repository"
	"kicau.local/edge/internal/streaming/service"
)

func TestSubscriptionsSeparateChannelAudioAndGlobalMetrics(t *testing.T) {
	catalog, err := cameraRepo.New([]cameraRepo.Input{{ID: "one", Source: "rtsp://192.0.2.1/a"}, {ID: "two", Source: "rtsp://192.0.2.1/b"}}, "rtsp://127.0.0.1:8554")
	if err != nil {
		t.Fatal(err)
	}
	broadcast := service.NewBroadcaster()
	uc := NewAudioUseCase(AudioOptions{}, catalog, broadcast, nil, nil)
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
	uc.Ingest("one", make([]byte, 6400))
	select {
	case <-audio:
	default:
		t.Fatal("audio frame missed")
	}
	select {
	case <-metrics:
	default:
		t.Fatal("metric missed")
	}
}
