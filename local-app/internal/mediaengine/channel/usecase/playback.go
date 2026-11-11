package usecase

import "context"

// LocalPlayback validates the catalog; delivery separately enforces the LAN boundary.
// It deliberately does not change the cloud-ticket authorization implementation.
type LocalPlayback struct {
	catalog interface{ HubRTSP(string) (string, error) }
}

func NewLocalPlayback(c interface{ HubRTSP(string) (string, error) }) *LocalPlayback {
	return &LocalPlayback{c}
}
func (p *LocalPlayback) ValidateChannel(id string) error { _, e := p.catalog.HubRTSP(id); return e }
func (p *LocalPlayback) Authorize(_ context.Context, id, _ string) error {
	return p.ValidateChannel(id)
}
func (p *LocalPlayback) RequiresTicket() bool { return false }
