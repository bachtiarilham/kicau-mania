package usecase

// This file creates consistent database snapshots and tracks backup outcomes.

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"kicau.local/edge/internal/apperror"
)

func (u *UseCase) Backup(ctx context.Context) (string, error) {
	if u.opts.BackupDir == "" {
		return "", apperror.Public(apperror.ErrUnavailable, "BACKUP_DIR belum dikonfigurasi pada penyimpanan terpisah.")
	}
	name := filepath.Join(u.opts.BackupDir, fmt.Sprintf("arena-%s-%s-%s.sqlite", u.opts.EngineID, time.Now().UTC().Format("20060102T150405"), operationID()))
	err := u.repo.Backup(ctx, name)
	u.healthMu.Lock()
	u.backupFailed = err != nil
	if err == nil {
		u.lastBackup = time.Now().UTC()
	}
	u.healthMu.Unlock()
	return name, err
}
