package server

import (
	"errors"
	"reflect"
	"testing"
)

type trackedModule struct {
	name   string
	events *[]string
	fail   bool
	open   bool
}

func (m *trackedModule) Start() error {
	*m.events = append(*m.events, "start:"+m.name)
	m.open = true
	if m.fail {
		return errors.New("startup failed")
	}
	return nil
}
func (m *trackedModule) Close() { *m.events = append(*m.events, "close:"+m.name); m.open = false }
func TestMediaStartupFailureDoesNotCloseHealthyModules(t *testing.T) {
	events := []string{}
	saving := &trackedModule{"saving", &events, false, false}
	video := &trackedModule{"video", &events, true, false}
	audio := &trackedModule{"audio", &events, false, false}
	stop := startIndependentModules(saving, video, audio)
	want := []string{"start:saving", "start:video", "close:video", "start:audio"}
	if !reflect.DeepEqual(events, want) {
		t.Fatal(events)
	}
	if !saving.open || video.open || !audio.open {
		t.Fatal("failure disabled a healthy module")
	}
	stop()
}
func TestShutdownFinalizesSavingFirstAndIsIdempotent(t *testing.T) {
	events := []string{}
	saving := &trackedModule{"saving", &events, false, false}
	video := &trackedModule{"video", &events, false, false}
	audio := &trackedModule{"audio", &events, false, false}
	stop := startIndependentModules(saving, video, audio)
	stop()
	stop()
	want := []string{"start:saving", "start:video", "start:audio", "close:saving", "close:video", "close:audio"}
	if !reflect.DeepEqual(events, want) {
		t.Fatal(events)
	}
}
