package repository

import (
	"fmt"
	"net/url"
	"strings"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/camera/model"
)

type Input struct {
	ID     string
	Source string
}

type Catalog struct {
	inputs []Input
	byID   map[string]Input
	base   string
}

type catalogURLError struct {
	message string
	cause   error
}

func (e *catalogURLError) Error() string {
	return e.message + ": invalid URL syntax (check percent-encoding)"
}
func (e *catalogURLError) Unwrap() error { return e.cause }

func New(inputs []Input, rtspBase string) (*Catalog, error) {
	base, e := url.Parse(rtspBase)
	if e != nil {
		return nil, &catalogURLError{message: "invalid local RTSP hub address", cause: e}
	}
	if base.Scheme != "rtsp" || base.Host == "" {
		return nil, fmt.Errorf("invalid local RTSP hub address")
	}
	if len(inputs) < 1 || len(inputs) > 24 {
		return nil, fmt.Errorf("NVR requires 1..24 channels")
	}
	c := &Catalog{inputs: append([]Input(nil), inputs...), byID: map[string]Input{}, base: strings.TrimRight(rtspBase, "/")}
	for _, in := range inputs {
		if !identifier.Valid(in.ID) {
			return nil, fmt.Errorf("invalid NVR channel ID")
		}
		if _, ok := c.byID[in.ID]; ok {
			return nil, fmt.Errorf("duplicate NVR channel ID")
		}
		u, e := url.Parse(in.Source)
		if e != nil {
			return nil, &catalogURLError{message: fmt.Sprintf("invalid NVR source for channel %s", in.ID), cause: e}
		}
		if u.Host == "" || (u.Scheme != "rtsp" && u.Scheme != "rtsps") {
			return nil, fmt.Errorf("invalid NVR source for channel %s", in.ID)
		}
		c.byID[in.ID] = in
	}
	return c, nil
}

// Inputs is reserved for infrastructure configuration; it contains credentials.
func (c *Catalog) Inputs() []Input { return append([]Input(nil), c.inputs...) }
func (c *Catalog) Channels() []model.Channel {
	out := make([]model.Channel, 0, len(c.inputs))
	for _, in := range c.inputs {
		out = append(out, model.Channel{ID: in.ID})
	}
	return out
}

// HubRTSP resolves a channel to the local MediaMTX RTSP URL, never the upstream.
func (c *Catalog) HubRTSP(id string) (string, error) {
	_, ok := c.byID[id]
	if !ok {
		return "", apperror.ErrNotFound
	}
	return c.base + "/" + id, nil
}

func (c *Catalog) Source(id string) (string, error) {
	in, ok := c.byID[id]
	if !ok {
		return "", apperror.ErrNotFound
	}
	return in.Source, nil
}
