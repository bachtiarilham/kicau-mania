package usecase

// This file prepares and validates event snapshots from cloud or local imports.

import (
	"context"
	"encoding/json"

	"golang.org/x/crypto/bcrypt"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/core/identifier"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
)

// Pull is an explicit preparation operation, never part of scoring or startup.
func (u *UseCase) Pull(ctx context.Context, eventID string) error {
	if !identifier.Valid(eventID) {
		return invalid("ID event tidak valid.")
	}
	if u.opts.PackageSource == nil {
		return apperror.Public(apperror.ErrUnavailable, "CLOUD_PACKAGES_URL belum dikonfigurasi.")
	}
	data, err := u.opts.PackageSource.Fetch(ctx, eventID)
	if err != nil {
		return apperror.Public(apperror.ErrUnavailable, "Cloud tidak tersedia. Paket lokal yang sudah disiapkan tetap dapat digunakan.")
	}
	var bundle model.Bundle
	if json.Unmarshal(data, &bundle) != nil || bundle.ID != eventID {
		return invalid("Paket cloud tidak valid.")
	}
	return u.Prepare(ctx, bundle)
}

func (u *UseCase) Prepare(ctx context.Context, bundle model.Bundle) error {
	u.ops.Lock()
	defer u.ops.Unlock()
	if err := u.validateBundle(&bundle); err != nil {
		return err
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	fingerprint := hash(data)
	return u.repo.Update(ctx, bundle.ID, func(e *model.Event) ([]exchange.Change, error) {
		if e.Bundle.ID != "" {
			if e.Fingerprint == fingerprint {
				return nil, nil
			}
			if bundle.Version <= e.Bundle.Version {
				return nil, conflict("Versi paket event harus lebih baru.")
			}
			for _, round := range e.Bundle.Rounds {
				if round.State != model.Prepared {
					return nil, conflict("Paket event sudah digunakan dan tidak dapat diganti.")
				}
			}
		}
		*e = model.Event{Bundle: bundle, Fingerprint: fingerprint, Revisions: []model.Revision{}, Audit: []model.Audit{}}
		// Credentials never leave the local credential store through the outbox.
		public := bundle
		public.Judges = append([]model.Judge(nil), bundle.Judges...)
		for i := range public.Judges {
			public.Judges[i].PasswordHash = ""
		}
		v, err := change(e, "event.prepared", public)
		return []exchange.Change{v}, err
	})
}

func (u *UseCase) validateBundle(b *model.Bundle) error {
	if !identifier.Valid(b.ID) || b.Name == "" || len(b.Name) > 200 || b.Version < 1 || b.ArenaID != u.opts.ArenaID || b.EngineID != u.opts.EngineID {
		return invalid("Identitas, versi, arena, atau engine paket tidak sesuai.")
	}
	if len(b.Judges) == 0 || len(b.Criteria) == 0 || len(b.Rounds) == 0 {
		return invalid("Paket wajib memuat juri, kriteria, dan ronde.")
	}
	judges, usernames, criteria, roundIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, j := range b.Judges {
		cost, err := bcrypt.Cost([]byte(j.PasswordHash))
		if !identifier.Valid(j.ID) || !identifier.Valid(j.Username) || j.Name == "" || judges[j.ID] || usernames[j.Username] || err != nil || cost < 10 || cost > 16 {
			return invalid("Juri wajib unik dan memiliki hash password event bcrypt cost 10–16.")
		}
		judges[j.ID] = true
		usernames[j.Username] = true
	}
	for _, c := range b.Criteria {
		if !identifier.Valid(c.ID) || c.Name == "" || criteria[c.ID] {
			return invalid("Kriteria tidak valid atau duplikat.")
		}
		criteria[c.ID] = true
	}
	for i := range b.Rounds {
		r := &b.Rounds[i]
		if !identifier.Valid(r.ID) || roundIDs[r.ID] || len(r.Participants) != 24 || (r.State != "" && r.State != model.Prepared) {
			return invalid("Setiap ronde wajib unik, berstatus PREPARED, dan memiliki tepat 24 peserta/lapak.")
		}
		roundIDs[r.ID] = true
		r.State = model.Prepared
		r.Reason = ""
		if r.Name == "" {
			r.Name = r.ID
		}
		if len(r.JudgeIDs) == 0 {
			for _, j := range b.Judges {
				r.JudgeIDs = append(r.JudgeIDs, j.ID)
			}
		}
		seenJudges, participants, lapaks, channels := map[string]bool{}, map[string]bool{}, map[int]bool{}, map[string]bool{}
		for _, j := range r.JudgeIDs {
			if !judges[j] || seenJudges[j] {
				return invalid("Penugasan juri ronde tidak valid.")
			}
			seenJudges[j] = true
		}
		for _, p := range r.Participants {
			if !identifier.Valid(p.ID) || !identifier.Valid(p.ChannelID) || p.Name == "" || p.Lapak < 1 || p.Lapak > 24 || participants[p.ID] || lapaks[p.Lapak] || channels[p.ChannelID] {
				return invalid("Peserta, channel, dan nomor lapak 1–24 harus unik pada setiap ronde.")
			}
			participants[p.ID] = true
			lapaks[p.Lapak] = true
			channels[p.ChannelID] = true
		}
	}
	return nil
}
