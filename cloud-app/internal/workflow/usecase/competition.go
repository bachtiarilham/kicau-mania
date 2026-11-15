package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	input "kicau.cloud/control/internal/workflow/model"
	"math/big"
	"regexp"
	"sort"
	"strings"
)

func (u *UseCase) Registration(c context.Context, actor, id, arena string, f model.Record) (model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		return nil, e
	}
	fields := "user_id round_id bird_id gantangan_id entry_fee payment_method"
	if e := validPatch(f, fields, fields, "user_id round_id bird_id gantangan_id", "", true); e != nil {
		return nil, e
	}
	cents, e := model.Cents(f["entry_fee"])
	if e != nil || cents < 0 || cents > 99999999999999 {
		return nil, apperror.Invalid("Biaya tiket tidak valid")
	}
	if str(f, "payment_method") != "CASH" && str(f, "payment_method") != "TRANSFER" {
		return nil, apperror.ErrInvalid
	}
	if id != "" && !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	e = u.db.Do(c, func(c context.Context) error {
		a, e := u.db.One(c, "ArenaLock", arena)
		if e != nil {
			return e
		}
		if !model.Flag(a, "is_active") {
			return apperror.ErrConflict
		}
		var old model.Record
		if id != "" {
			old, e = u.db.One(c, "RegistrationGet", arena, id)
			if e != nil {
				return e
			}
		}
		locks := []string{str(f, "round_id")}
		if id != "" && str(old, "round_id") != locks[0] {
			locks = append(locks, str(old, "round_id"))
		}
		sort.Strings(locks)
		for _, r := range locks {
			row, e := u.db.One(c, "RoundLock", arena, r)
			if e != nil {
				return e
			}
			if str(row, "status") != "PREPARED" {
				return apperror.Public(apperror.ErrConflict, "Tiket hanya dapat diubah sebelum ronde dimulai")
			}
		}
		if id != "" {
			old, e = u.db.One(c, "RegistrationLock", arena, id)
			if e != nil {
				return e
			}
			if str(old, "status") != "PENDING_PAYMENT" || str(old, "payment_status") == "VERIFIED" {
				return apperror.ErrConflict
			}
		}
		if _, e = u.activeUser(c, str(f, "user_id")); e != nil {
			return e
		}
		b, e := u.db.One(c, "BirdLock", str(f, "bird_id"))
		if e != nil {
			return e
		}
		if b["deleted_at"] != nil {
			return apperror.ErrConflict
		}
		if _, e = u.activeUser(c, str(b, "owner_id")); e != nil {
			return e
		}
		r, e := u.db.One(c, "RoundGet", arena, str(f, "round_id"))
		if e != nil {
			return e
		}
		if str(r, "bird_species_id") != str(b, "bird_species_id") {
			return apperror.Invalid("Jenis burung tidak sesuai ronde")
		}
		if _, e = u.db.One(c, "GantanganGet", arena, str(f, "gantangan_id")); e != nil {
			return e
		}
		var row model.Record
		if id == "" {
			row, e = u.db.One(c, "RegistrationCreate", str(f, "user_id"), arena, str(f, "round_id"), str(f, "bird_id"), str(f, "gantangan_id"), fmt.Sprint(f["entry_fee"]), str(f, "payment_method"))
		} else {
			row, e = u.db.One(c, "RegistrationUpdate", arena, id, str(f, "user_id"), str(f, "round_id"), str(f, "bird_id"), str(f, "gantangan_id"), fmt.Sprint(f["entry_fee"]), str(f, "payment_method"))
		}
		if e == nil {
			id = str(row, "id")
		}
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.Get(c, actor, "registrations", id, arena)
}
func (u *UseCase) TicketAction(c context.Context, actor, arena, id, action, reason string) (model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		return nil, e
	}
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	e := u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
			return e
		}
		t, e := u.db.One(c, "RegistrationGet", arena, id)
		if e != nil {
			return e
		}
		r, e := u.db.One(c, "RoundLock", arena, str(t, "round_id"))
		if e != nil {
			return e
		}
		t, e = u.db.One(c, "RegistrationLock", arena, id)
		if e != nil {
			return e
		}
		if str(r, "status") == "CLOSED" || str(r, "status") == "CANCELLED" {
			return apperror.ErrConflict
		}
		q := ""
		args := []any{arena, id}
		switch action {
		case "verify":
			if str(r, "status") != "PREPARED" {
				return apperror.ErrConflict
			}
			if str(t, "status") == "CONFIRMED" {
				return nil
			}
			proof, e := u.db.One(c, "RegistrationGet", arena, id)
			if e != nil {
				return e
			}
			if str(proof, "payment_method") == "TRANSFER" && !model.Flag(proof, "has_payment_proof") {
				return apperror.Public(apperror.ErrConflict, "Unggah bukti transfer sebelum verifikasi")
			}
			q = "RegistrationVerifyPayment"
		case "reject":
			if str(r, "status") != "PREPARED" {
				return apperror.ErrConflict
			}
			q = "RegistrationRejectPayment"
		case "cancel":
			if str(t, "status") == "CANCELLED" {
				return nil
			}
			if str(r, "status") != "PREPARED" {
				return apperror.ErrConflict
			}
			q = "RegistrationCancel"
		case "disqualify":
			if strings.TrimSpace(reason) == "" || len(reason) > 2000 {
				return apperror.Invalid("Alasan diskualifikasi wajib diisi")
			}
			if str(t, "participant_status") == "DISQUALIFIED" {
				return nil
			}
			q = "ParticipantDisqualify"
			args = append(args, reason)
		case "requalify":
			if str(t, "participant_status") == "ACTIVE" {
				return nil
			}
			q = "ParticipantRequalify"
		default:
			return apperror.ErrNotFound
		}
		if str(t, "status") == "CANCELLED" {
			return apperror.ErrConflict
		}
		_, e = u.db.One(c, q, args...)
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.Get(c, actor, "registrations", id, arena)
}
func (u *UseCase) Proof(c context.Context, actor, arena, id string, bytes []byte, mime string) (model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		return nil, e
	}
	if len(bytes) == 0 || len(bytes) > 5*1024*1024 || (mime != "image/jpeg" && mime != "image/png") {
		return nil, apperror.ErrInvalid
	}
	e := u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
			return e
		}
		t, e := u.db.One(c, "RegistrationGet", arena, id)
		if e != nil {
			return e
		}
		r, e := u.db.One(c, "RoundLock", arena, str(t, "round_id"))
		if e != nil {
			return e
		}
		if str(r, "status") != "PREPARED" {
			return apperror.ErrConflict
		}
		if _, e = u.db.One(c, "RegistrationLock", arena, id); e != nil {
			return e
		}
		_, e = u.db.One(c, "RegistrationSetProof", arena, id, bytes, mime)
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.Get(c, actor, "registrations", id, arena)
}
func (u *UseCase) ReadProof(c context.Context, actor, arena, id string) ([]byte, string, error) {
	if _, e := u.Get(c, actor, "registrations", id, arena); e != nil {
		return nil, "", e
	}
	r, e := u.db.One(c, "RegistrationProofInternal", arena, id)
	if e != nil {
		return nil, "", e
	}
	b, e := base64.StdEncoding.DecodeString(str(r, "payment_proof_base64"))
	if e != nil || len(b) == 0 {
		return nil, "", apperror.ErrNotFound
	}
	return b, str(r, "payment_proof_mime"), nil
}
func (u *UseCase) RoundAction(c context.Context, actor, arena, id, action, expected string) (model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		return nil, e
	}
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	e := u.db.Do(c, func(c context.Context) error {
		a, e := u.db.One(c, "ArenaLock", arena)
		if e != nil {
			return e
		}
		r, e := u.db.One(c, "RoundLock", arena, id)
		if e != nil {
			return e
		}
		state := str(r, "status")
		if expected != "" && state != expected {
			return apperror.ErrConflict
		}
		next := ""
		switch action {
		case "start":
			if state != "PREPARED" || !model.Flag(a, "is_active") {
				return apperror.ErrConflict
			}
			tickets, e := u.db.Many(c, "RoundParticipants", arena, id)
			if e != nil {
				return e
			}
			eligible := 0
			for _, t := range tickets {
				if str(t, "participant_status") == "ACTIVE" {
					eligible++
				}
			}
			judges, e := u.db.Many(c, "RoundJudgeList", arena, id)
			if e != nil {
				return e
			}
			criteria, e := u.db.Many(c, "CriteriaForArena", arena)
			if e != nil {
				return e
			}
			if eligible == 0 || len(judges) == 0 || len(criteria) == 0 {
				return apperror.Public(apperror.ErrConflict, "Peserta terkonfirmasi, juri dan kriteria wajib tersedia")
			}
			for _, j := range judges {
				if _, e = u.db.One(c, "JudgeAccountForArena", str(j, "judge_user_id"), arena); e != nil {
					return apperror.Public(apperror.ErrConflict, "Roster memuat juri yang tidak aktif atau penugasannya telah dicabut")
				}
			}
			if e = u.db.Exec(c, "ArenaLockCriteria", arena); e != nil {
				return e
			}
			next = "RUNNING"
		case "begin-judging":
			if state != "RUNNING" {
				return apperror.ErrConflict
			}
			next = "JUDGING"
		case "close":
			if state != "JUDGING" {
				return apperror.ErrConflict
			}
			complete, e := u.db.One(c, "RoundCompleteness", arena, id)
			if e != nil {
				return e
			}
			if !model.Flag(complete, "all_complete") {
				return apperror.Public(apperror.ErrConflict, "Penilaian peserta belum lengkap")
			}
			next = "CLOSED"
		case "cancel":
			if state == "CANCELLED" {
				return nil
			}
			if state == "CLOSED" {
				return apperror.ErrConflict
			}
			next = "CANCELLED"
		default:
			return apperror.ErrNotFound
		}
		_, e = u.db.One(c, "RoundSetStatus", arena, id, state, next)
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.Get(c, actor, "rounds", id, arena)
}
func (u *UseCase) RoundData(c context.Context, actor, arena, id, kind string) ([]model.Record, error) {
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		if kind == "judges" {
			return nil, e
		}
		if _, e = u.db.One(c, "JudgeAssignmentActive", arena, id, actor); e != nil {
			return nil, apperror.ErrForbidden
		}
	}
	q := map[string]string{"judges": "RoundJudgeList", "participants": "RoundParticipants", "recap": "RoundLiveSummary"}[kind]
	if q == "" {
		return nil, apperror.ErrNotFound
	}
	return u.db.Many(c, q, arena, id)
}
func (u *UseCase) AssignJudge(c context.Context, actor, arena, round, judge string, remove bool) ([]model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		return nil, e
	}
	if !model.UUID(round) || !model.UUID(judge) {
		return nil, apperror.ErrInvalid
	}
	e := u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
			return e
		}
		r, e := u.db.One(c, "RoundLock", arena, round)
		if e != nil {
			return e
		}
		if str(r, "status") != "PREPARED" {
			return apperror.ErrConflict
		}
		q := "RoundJudgeAssign"
		if remove {
			q = "RoundJudgeRemove"
		} else {
			if _, e = u.db.One(c, "JudgeAccountForArena", judge, arena); e != nil {
				return apperror.Invalid("Akun belum ditugaskan sebagai JURI di arena ini")
			}
		}
		return u.db.Exec(c, q, arena, round, judge)
	})
	if e != nil {
		return nil, e
	}
	return u.RoundData(c, actor, arena, round, "judges")
}
func (u *UseCase) Criteria(c context.Context, actor, id, arena string, f model.Record) (model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		return nil, e
	}
	if e := validPatch(f, "criteria_name", "criteria_name", "", "", true); e != nil {
		return nil, e
	}
	e := u.db.Do(c, func(c context.Context) error {
		a, e := u.db.One(c, "ArenaLock", arena)
		if e != nil {
			return e
		}
		if a["criteria_locked_at"] != nil {
			return apperror.Public(apperror.ErrConflict, "Kriteria arena sudah dikunci")
		}
		var r model.Record
		if id == "" {
			r, e = u.db.One(c, "ScoreCriterionCreate", arena, str(f, "criteria_name"))
			if e == nil {
				id = str(r, "id")
			}
		} else {
			_, e = u.db.One(c, "ScoreCriterionUpdate", arena, id, jsonValue(f))
		}
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.Get(c, actor, "criteria", id, arena)
}
func (u *UseCase) criteriaDelete(c context.Context, actor, id, arena string) error {
	a, e := u.db.One(c, "ArenaLock", arena)
	if e != nil {
		return e
	}
	if a["criteria_locked_at"] != nil {
		return apperror.ErrConflict
	}
	_, e = u.db.One(c, "ScoreCriterionDelete", arena, id)
	return e
}

var scorePattern = regexp.MustCompile(`^(?:[1-9](?:\.[0-9]+)?|10(?:\.0+)?)$`)

func (u *UseCase) Score(c context.Context, actor, arena, round, ticket string, scores []input.Score) ([]model.Record, error) {
	if !model.UUID(round) || !model.UUID(ticket) || len(scores) == 0 || len(scores) > 100 {
		return nil, apperror.ErrInvalid
	}
	if e := u.Require(c, actor, arena, "JURI"); e != nil {
		return nil, e
	}
	e := u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "JudgeAssignmentActive", arena, round, actor); e != nil {
			return apperror.ErrForbidden
		}
		r, e := u.db.One(c, "RoundLock", arena, round)
		if e != nil {
			return e
		}
		if str(r, "status") != "JUDGING" {
			return apperror.ErrConflict
		}
		t, e := u.db.One(c, "RegistrationLock", arena, ticket)
		if e != nil {
			return e
		}
		if str(t, "round_id") != round || str(t, "status") != "CONFIRMED" || str(t, "payment_status") != "VERIFIED" || str(t, "participant_status") != "ACTIVE" {
			return apperror.ErrConflict
		}
		cs, e := u.db.Many(c, "CriteriaForArena", arena)
		if e != nil {
			return e
		}
		if len(cs) != len(scores) {
			return apperror.Invalid("Seluruh kriteria harus diisi bersama")
		}
		allowed := map[string]bool{}
		for _, v := range cs {
			allowed[str(v, "id")] = true
		}
		for _, v := range scores {
			if len(v.Value) > 32 {
				return apperror.ErrInvalid
			}
			n, ok := new(big.Rat).SetString(v.Value)
			if !allowed[v.CriteriaID] || !scorePattern.MatchString(v.Value) || !ok || n.Cmp(big.NewRat(1, 1)) < 0 || n.Cmp(big.NewRat(10, 1)) > 0 {
				return apperror.ErrInvalid
			}
			delete(allowed, v.CriteriaID)
			if _, e = u.db.One(c, "ScoreUpsert", arena, round, ticket, actor, v.CriteriaID, v.Value); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return u.db.Many(c, "ScoreList", arena, ticket, actor)
}
func (u *UseCase) ReadScores(c context.Context, actor, arena, round, ticket string) ([]model.Record, error) {
	if _, e := u.db.One(c, "JudgeAssignmentActive", arena, round, actor); e != nil {
		return nil, apperror.ErrForbidden
	}
	t, e := u.Get(c, actor, "registrations", ticket, arena)
	if e != nil {
		return nil, e
	}
	if str(t, "round_id") != round {
		return nil, apperror.ErrNotFound
	}
	return u.db.Many(c, "ScoreList", arena, ticket, actor)
}
