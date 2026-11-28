package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/recording/model"
)

func (u *SessionUseCase) recordingRoot() string {
	if u.opts.RecordingsDir != "" {
		return u.opts.RecordingsDir
	}
	return filepath.Join(u.opts.DataDir, "recordings")
}

func (u *SessionUseCase) prepareRecordingLocation(ctx context.Context, session *model.Session) error {
	session.RecordingRoot = u.recordingRoot()
	round := session.Competition
	if round == nil {
		return u.checkRecordingDisk(session.RecordingRoot)
	}
	// Make the request immutable for the lifetime of the session.
	contextCopy := *round
	contextCopy.Gantangan = make(map[string]int, len(round.Gantangan))
	seen := make(map[int]bool, len(round.Gantangan))
	for _, channel := range session.Channels {
		slot := round.Gantangan[channel]
		if slot < 1 || slot > 24 || seen[slot] {
			return apperror.ErrInvalid
		}
		seen[slot] = true
		contextCopy.Gantangan[channel] = slot
	}
	session.Competition = &contextCopy
	date, err := time.Parse(time.DateOnly, round.EventDate)
	if err != nil || date.Format(time.DateOnly) != round.EventDate || round.RoundNumber < 1 ||
		!identifier.Valid(round.ArenaID) || !identifier.Valid(round.EventID) || !identifier.Valid(round.RoundID) ||
		strings.TrimSpace(round.ArenaName) == "" || strings.TrimSpace(round.EventName) == "" {
		return apperror.Public(apperror.ErrInvalid, "Nama arena, event, tanggal, dan nomor ronde diperlukan untuk menyimpan rekaman.")
	}
	sessions, err := u.journal.List(ctx)
	if err != nil {
		return err
	}
	var eventFolder string
	for _, previous := range sessions {
		old := previous.Competition
		if old == nil || old.ArenaID != round.ArenaID || old.EventID != round.EventID || previous.RecordingFolder == "" {
			continue
		}
		if old.RoundID == round.RoundID {
			if previous.RoundComplete {
				return apperror.Public(apperror.ErrConflict, "Ronde sudah selesai.")
			}
			session.RecordingRoot = previous.RecordingRoot
			session.RecordingFolder = previous.RecordingFolder
		}
		if previous.RecordingRoot == u.recordingRoot() {
			eventFolder = filepath.Dir(previous.RecordingFolder)
		}
	}
	if err := u.checkRecordingDisk(session.RecordingRoot); err != nil {
		return err
	}
	if session.RecordingFolder == "" {
		if eventFolder == "" {
			label := folderLabel(round.ArenaName) + "_" + folderLabel(round.EventName) + "_" + round.EventDate
			eventFolder, err = reserveRecordingFolder(session.RecordingRoot, label)
			if err != nil {
				return err
			}
		}
		session.RecordingFolder, err = reserveRecordingFolder(eventFolder, fmt.Sprintf("Ronde %02d", round.RoundNumber))
		if err != nil {
			return err
		}
	}
	return os.MkdirAll(filepath.Join(session.RecordingFolder, ".capture-"+session.ID), 0700)
}

func folderLabel(raw string) string {
	name := strings.Trim(strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune("<>:\"/\\|?*", r) {
			return '_'
		}
		return r
	}, raw), " .")
	runes := []rune(name)
	if len(runes) > 50 {
		name = strings.TrimRight(string(runes[:50]), " .")
	}
	if name == "" {
		return "_"
	}
	return name
}

func reserveRecordingFolder(parent, name string) (string, error) {
	for index := 1; ; index++ {
		folder := filepath.Join(parent, name)
		if index > 1 {
			folder = filepath.Join(parent, fmt.Sprintf("%s (%d)", name, index))
		}
		err := os.Mkdir(folder, 0700)
		if !errors.Is(err, os.ErrExist) {
			return folder, err
		}
	}
}

func (u *SessionUseCase) publishCompetitionRecording(ctx context.Context, rec model.Recording, parts []string, complete bool, finishErr error) error {
	if rec.PendingFile == "" {
		rec.VideoIssue = finishErr != nil || len(rec.Gaps) > 0
		if finishErr != nil {
			candidate := largestRecordingPart(append(parts, rec.File))
			if candidate == "" {
				rec.State = "UNAVAILABLE"
				rec.Error = "Rekaman tidak tersedia"
				return u.recordings.Save(ctx, rec)
			}
			if candidate != rec.File {
				if err := os.Rename(candidate, rec.File); err != nil {
					return err
				}
			}
		}
		suffix := ""
		if rec.VideoIssue {
			suffix = " - Rekaman Terbatas"
		}
		rec.PendingFile = rec.FinalBase + suffix + ".mkv"
		rec.DisplayFilename = filepath.Base(rec.PendingFile)
		rec.State = "FINALIZING"
		if err := u.recordings.Save(ctx, rec); err != nil {
			return err
		}
	}
	if !u.safeRecordingPath(rec, rec.PendingFile) {
		return apperror.ErrInvalid
	}
	if _, err := os.Stat(rec.File); err == nil {
		if err := os.Rename(rec.File, rec.PendingFile); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if info, err := os.Stat(rec.PendingFile); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return apperror.ErrUnavailable
	}

	previous, err := u.recordings.List(ctx)
	if err != nil {
		return err
	}
	for _, old := range previous {
		if old.ID == rec.ID || old.FinalBase != rec.FinalBase || old.FinalBase == "" || old.Superseded {
			continue
		}
		if old.State == "HELD" && old.File != rec.PendingFile && u.safeRecordingPath(old, old.File) {
			if err := os.Remove(old.File); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		old.Superseded = true
		if err := u.recordings.Save(ctx, old); err != nil {
			return err
		}
	}
	rec.File, rec.PendingFile = rec.PendingFile, ""
	rec.State = "HELD"
	if complete {
		rec.State = "READY"
	}
	rec.Error = ""
	return u.recordings.Save(ctx, rec)
}

func largestRecordingPart(parts []string) string {
	var largest int64
	best := ""
	for _, part := range parts {
		info, err := os.Stat(part)
		if err == nil && info.Mode().IsRegular() && info.Size() > largest {
			best, largest = part, info.Size()
		}
	}
	return best
}

func (u *SessionUseCase) cleanCompetitionCapture(session model.Session) {
	if session.Competition != nil && identifier.Valid(session.ID) && session.RecordingFolder != "" &&
		u.safeRecordingPath(model.Recording{StorageRoot: session.RecordingRoot}, session.RecordingFolder) {
		_ = os.RemoveAll(filepath.Join(session.RecordingFolder, ".capture-"+session.ID))
	}
}
