// Package usecase coordinates schema-010 business rules and transactions.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	input "kicau.cloud/control/internal/workflow/model"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
)

type Data interface {
	One(context.Context, string, ...any) (model.Record, error)
	Many(context.Context, string, ...any) ([]model.Record, error)
	Exec(context.Context, string, ...any) error
	Do(context.Context, func(context.Context) error) error
}
type Passwords interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
}
type UseCase struct {
	db        Data
	passwords Passwords
}

func New(d Data, p Passwords) *UseCase { return &UseCase{db: d, passwords: p} }
func null(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func str(r model.Record, k string) string { return model.Text(r, k) }
func num(r model.Record, k string) int    { n, _ := strconv.Atoi(fmt.Sprint(r[k])); return n }
func has(rows []model.Record, roles ...string) bool {
	for _, r := range rows {
		for _, role := range roles {
			if str(r, "name") == role {
				return true
			}
		}
	}
	return false
}
func (u *UseCase) Profile(c context.Context, id string) (model.Record, error) {
	return u.db.One(c, "ActorProfile", id)
}
func (u *UseCase) Admin(c context.Context, id string) error {
	r, e := u.Profile(c, id)
	if e != nil {
		return e
	}
	for _, v := range r["roles"].([]any) {
		if v == "ADMIN" {
			return nil
		}
	}
	for _, v := range r["arena_roles"].([]any) {
		if v.(map[string]any)["role"] == "ADMIN" {
			return nil
		}
	}
	return apperror.ErrForbidden
}
func (u *UseCase) Staff(c context.Context, id string) error {
	r, e := u.Profile(c, id)
	if e != nil {
		return e
	}
	for _, v := range r["roles"].([]any) {
		if v == "ADMIN" {
			return nil
		}
	}
	for _, v := range r["arena_roles"].([]any) {
		x := v.(map[string]any)["role"]
		if x == "ADMIN" || x == "OPERATOR" {
			return nil
		}
	}
	return apperror.ErrForbidden
}
func (u *UseCase) Require(c context.Context, id, arena string, roles ...string) error {
	if !model.UUID(arena) {
		return apperror.ErrForbidden
	}
	rows, e := u.db.Many(c, "ActorArenaRoles", id, arena)
	if e != nil {
		return e
	}
	if !has(rows, roles...) {
		return apperror.ErrForbidden
	}
	return nil
}
func (u *UseCase) manageAccount(c context.Context, actor, target string) error {
	if e := u.Staff(c, actor); e != nil {
		return e
	}
	r, e := u.db.One(c, "PrivilegedAccount", target, actor)
	if e != nil {
		return e
	}
	if model.Flag(r, "privileged") {
		if e = u.Admin(c, actor); e != nil {
			return e
		}
		if !model.Flag(r, "manageable") && actor != target {
			return apperror.ErrForbidden
		}
	}
	return nil
}
func (u *UseCase) lastAdmin(c context.Context, target string) error {
	scopes, e := u.db.Many(c, "AccountAdminScopes", target)
	if e != nil {
		return e
	}
	for _, s := range scopes {
		if _, e = u.db.One(c, "ArenaLock", str(s, "id")); e != nil {
			return e
		}
		r, e := u.db.One(c, "AssignmentCount", str(s, "id"))
		if e != nil {
			return e
		}
		if num(r, "count") <= 1 {
			return apperror.Public(apperror.ErrConflict, "Admin terakhir arena tidak dapat dinonaktifkan atau dicabut")
		}
	}
	return nil
}
func (u *UseCase) context(c context.Context, actor, arena string) error {
	_, e := u.db.One(c, "SetMutationContext", actor, null(arena))
	return e
}
func (u *UseCase) version(c context.Context, query, id string, v int) (model.Record, error) {
	if v < 1 {
		return nil, apperror.Invalid("expected_version wajib diisi")
	}
	r, e := u.db.One(c, query, id)
	if e != nil {
		return nil, e
	}
	if num(r, "version") != v {
		return nil, apperror.Public(apperror.ErrConflict, "Data telah berubah. Muat ulang sebelum menyimpan")
	}
	return r, nil
}
func validID(id string) error {
	if !model.UUID(id) {
		return apperror.ErrInvalid
	}
	return nil
}
func validPatch(r model.Record, allowed, required, ids, positive string, create bool) error {
	if e := model.ValidateFields(r, allowed); e != nil {
		return e
	}
	if e := model.Strings(r, "name title code description address timezone phone email event_date status criteria_name award_name event_name organizer_name awarded_at certificate_url payment_method"); e != nil {
		return e
	}
	copy := model.Record{}
	for k, v := range r {
		copy[k] = v
	}
	if copy["rank"] == nil {
		delete(copy, "rank")
	}
	for _, k := range []string{"description", "address"} {
		if text, ok := copy[k].(string); ok {
			if len(text) > 4096 || strings.ContainsRune(text, 0) {
				return apperror.ErrInvalid
			}
			copy[k] = strings.NewReplacer("\r", " ", "\n", " ").Replace(text)
		}
	}
	if v, ok := r["expected_version"]; ok {
		n, valid := v.(json.Number)
		if !valid {
			return apperror.ErrInvalid
		}
		i, e := n.Int64()
		if e != nil || i < 1 {
			return apperror.ErrInvalid
		}
	}
	return model.Validate(copy, required, ids, positive, "", create)
}
func email(v any) error {
	if v == nil || v == "" {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return apperror.ErrInvalid
	}
	a, e := mail.ParseAddress(s)
	if e != nil || a.Address != s {
		return apperror.Invalid("Email tidak valid")
	}
	return nil
}
func urlValue(v any) error {
	if v == nil || v == "" {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return apperror.ErrInvalid
	}
	a, e := url.Parse(s)
	if e != nil || (a.Scheme != "https" && a.Scheme != "http") || a.Hostname() == "" || a.User != nil {
		return apperror.Invalid("URL sertifikat tidak valid")
	}
	return nil
}
func jsonValue(v any) string { b, _ := json.Marshal(v); return string(b) }
func (u *UseCase) scoped(c context.Context, actor, res, arena string, write bool) error {
	if res == "criteria" && !write {
		return u.Require(c, actor, arena, "ADMIN", "OPERATOR", "JURI")
	}
	switch res {
	case "users", "birds", "bird-species":
		return u.Staff(c, actor)
	case "arenas":
		if write {
			if arena == "" {
				return u.Admin(c, actor)
			}
			return u.Require(c, actor, arena, "ADMIN")
		}
		return nil
	default:
		return u.Require(c, actor, arena, "ADMIN", "OPERATOR")
	}
}
func (u *UseCase) List(c context.Context, actor, res string, f input.Filter) ([]model.Record, error) {
	if len(f.Search) > 200 {
		return nil, apperror.ErrInvalid
	}
	if f.Limit < 1 || f.Limit > 100 || f.Cursor != "" && !model.UUID(f.Cursor) {
		return nil, apperror.ErrInvalid
	}
	for _, id := range []string{f.Arena, f.Event, f.Round, f.Owner, f.Bird} {
		if id != "" && !model.UUID(id) {
			return nil, apperror.ErrInvalid
		}
	}
	if res == "judge-rounds" {
		return u.db.Many(c, "JudgeRoundList", actor, null(f.Cursor), f.Limit+1)
	}
	if res == "birds" {
		if e := u.Staff(c, actor); e != nil {
			f.Owner = actor
		}
		return u.db.Many(c, "BirdFilteredList", null(f.Owner), null(f.Cursor), f.Limit+1, null(f.Search))
	}
	if res == "registrations" {
		if e := u.Require(c, actor, f.Arena, "ADMIN", "OPERATOR"); e != nil {
			if !errors.Is(e, apperror.ErrForbidden) {
				return nil, e
			}
			if staff := u.Staff(c, actor); staff == nil {
				return nil, apperror.ErrForbidden
			} else if !errors.Is(staff, apperror.ErrForbidden) {
				return nil, staff
			}
			if j := u.Require(c, actor, f.Arena, "JURI"); j == nil && f.Round != "" {
				if _, j = u.db.One(c, "JudgeAssignmentActive", f.Arena, f.Round, actor); j != nil {
					return nil, apperror.ErrForbidden
				}
			} else {
				return u.db.Many(c, "ParticipantRegistrations", actor, null(f.Cursor), f.Limit+1, null(f.Arena))
			}
		}
		return u.db.Many(c, "RegistrationFilteredList", f.Arena, null(f.Round), null(f.Event), null(f.Cursor), f.Limit+1, null(f.Bird))
	}
	if e := u.scoped(c, actor, res, f.Arena, false); e != nil {
		return nil, e
	}
	switch res {
	case "arenas":
		return u.db.Many(c, "ActorArenas", actor, null(f.Cursor), f.Limit+1)
	case "users":
		var eo any
		if f.EO != "" {
			eo = f.EO == "true"
		}
		return u.db.Many(c, "AccountList", null(f.Cursor), f.Limit+1, eo, null(f.Role), null(f.Search))
	case "bird-species":
		return u.db.Many(c, "BirdSpeciesList", null(f.Cursor), f.Limit+1)
	case "rounds":
		return u.db.Many(c, "RoundFilteredList", f.Arena, null(f.Event), null(f.Cursor), f.Limit+1)
	case "arena-judges":
		return u.db.Many(c, "ArenaJudgeLookup", f.Arena, null(f.Cursor), f.Limit+1)
	case "events", "categories", "gantangan", "criteria":
		prefix := map[string]string{"events": "Event", "categories": "Category", "gantangan": "Gantangan", "criteria": "ScoreCriterion"}[res]
		return u.db.Many(c, prefix+"List", f.Arena, null(f.Cursor), f.Limit+1)
	}
	return nil, apperror.ErrNotFound
}
func (u *UseCase) Get(c context.Context, actor, res, id, arena string) (model.Record, error) {
	if e := validID(id); e != nil {
		return nil, e
	}
	var r model.Record
	var e error
	switch res {
	case "users":
		r, e = u.db.One(c, "MasterUserGet", id)
	case "birds":
		r, e = u.db.One(c, "BirdGet", id)
	case "arenas":
		r, e = u.db.One(c, "ArenaGet", id)
		arena = id
	case "bird-species":
		r, e = u.db.One(c, "BirdSpeciesGet", id)
	default:
		if !model.UUID(arena) {
			return nil, apperror.ErrInvalid
		}
		p := map[string]string{"events": "Event", "categories": "Category", "rounds": "Round", "registrations": "Registration", "gantangan": "Gantangan", "criteria": "ScoreCriterion"}[res]
		if p == "" {
			return nil, apperror.ErrNotFound
		}
		r, e = u.db.One(c, p+"Get", arena, id)
	}
	if e != nil {
		return nil, e
	}
	if e = u.scoped(c, actor, res, arena, false); e != nil {
		if res == "rounds" {
			if _, j := u.db.One(c, "JudgeAssignmentActive", arena, id, actor); j == nil {
				return r, nil
			}
		}
		if res == "users" && id == actor || res == "birds" && str(r, "owner_id") == actor || res == "registrations" && str(r, "user_id") == actor {
			return r, nil
		}
		if res == "registrations" {
			if _, j := u.db.One(c, "JudgeAssignmentActive", arena, str(r, "round_id"), actor); j == nil {
				return r, nil
			}
		}
		return nil, e
	}
	if res == "arenas" {
		if e = u.Require(c, actor, arena, "ADMIN", "OPERATOR", "JURI", "PESERTA"); e != nil {
			return nil, e
		}
	}
	return r, nil
}
