package model

import (
	"errors"
	"fmt"
	"regexp"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

var (
	ErrInvalidChannel     = errors.New("invalid zoom channel ID")
	ErrInvalidCrop        = errors.New("crop dimensions must be positive")
	ErrInvalidPerspective = errors.New("perspective requires 4 valid coordinate points")
)

type CropConfig struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Point [2]int

type PerspectiveConfig struct {
	Enabled   bool     `json:"enabled"`
	SrcPoints [4]Point `json:"src_points,omitempty"`
}

type ZoomChannel struct {
	ChannelID    string            `json:"channel_id"`
	Area         string            `json:"area"`
	Active       bool              `json:"active"`
	SourceCamera string            `json:"source_camera"`
	LineOfSight  string            `json:"line_of_sight,omitempty"`
	Crop         CropConfig        `json:"crop"`
	Perspective  PerspectiveConfig `json:"perspective"`
}

func (c *ZoomChannel) Validate() error {
	if !safeID.MatchString(c.ChannelID) {
		return ErrInvalidChannel
	}
	if c.Crop.Width <= 0 || c.Crop.Height <= 0 || c.Crop.X < 0 || c.Crop.Y < 0 {
		return ErrInvalidCrop
	}
	if c.Perspective.Enabled {
		for i, pt := range c.Perspective.SrcPoints {
			if pt[0] < 0 || pt[1] < 0 {
				return fmt.Errorf("%w: point %d has negative coordinates", ErrInvalidPerspective, i)
			}
		}
	}
	return nil
}

type ZoomMapping struct {
	ActiveChannels []ZoomChannel `json:"active_channels"`
}
