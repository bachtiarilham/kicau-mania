// Recovery reconciles recording manifests when the engine starts.
package usecase

import (
	"context"
	"path/filepath"
	"time"
)

func (u *UseCase) recover(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if err := u.disk.Prepare(filepath.Join(u.cfg.DataDir, "recordings")); err != nil {
		return err
	}
	records, err := u.repo.List(ctx)
	if err != nil {
		return err
	}
	// The final filename is the commit marker for a cleanly finalized MP4.
	for _, r := range records {
		if r.State == "RECORDING" {
			r.State = "INTERRUPTED"
			r.EndedAt = time.Now()
			if size, err := u.disk.Size(r.File); err == nil && size > 0 {
				r.State = "READY"
			}
			if err := u.repo.Save(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}
