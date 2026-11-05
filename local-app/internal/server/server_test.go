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
func TestStartupFailureClosesAllConstructedModules(t *testing.T) {
	events := []string{}
	saving := &trackedModule{"saving", &events, false, false}
	video := &trackedModule{"video", &events, true, false}
	audio := &trackedModule{"audio", &events, false, false}
	stop, e := startModules(saving, video, audio)
	if e == nil || stop != nil {
		t.Fatal("startup failure was ignored")
	}
	want := []string{"start:saving", "start:video", "close:audio", "close:video", "close:saving"}
	if !reflect.DeepEqual(events, want) {
		t.Fatal(events)
	}
	if saving.open || video.open || audio.open {
		t.Fatal("startup leaked a module")
	}
}
func TestShutdownFinalizesSavingFirstAndIsIdempotent(t *testing.T) {
	events := []string{}
	saving := &trackedModule{"saving", &events, false, false}
	video := &trackedModule{"video", &events, false, false}
	audio := &trackedModule{"audio", &events, false, false}
	stop, e := startModules(saving, video, audio)
	if e != nil {
		t.Fatal(e)
	}
	stop()
	stop()
	want := []string{"start:saving", "start:video", "start:audio", "close:saving", "close:video", "close:audio"}
	if !reflect.DeepEqual(events, want) {
		t.Fatal(events)
	}
}
