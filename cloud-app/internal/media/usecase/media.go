package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kicau.cloud/control/internal/apperror"
	core "kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/media/model"
)

type Data interface {
	One(context.Context, string, ...any) (core.Record, error)
	Many(context.Context, string, ...any) ([]core.Record, error)
	Exec(context.Context, string, ...any) error
	Do(context.Context, func(context.Context) error) error
}
type Access interface {
	Require(context.Context, string, string, ...string) error
}
type UseCase struct {
	DB      Data
	ACL     Access
	Objects model.Objects
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func text(r core.Record, k string) string { return core.Text(r, k) }
func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func parts(r core.Record) []model.Part {
	b, _ := json.Marshal(r["parts"])
	var p []model.Part
	_ = json.Unmarshal(b, &p)
	return p
}
func (u *UseCase) Pair(c context.Context, actor, id, arena string) (core.Record, error) {
	if !core.UUID(id) || !core.UUID(arena) {
		return nil, apperror.ErrInvalid
	}
	if e := u.ACL.Require(c, actor, arena, "ADMIN"); e != nil {
		return nil, e
	}
	var secret [32]byte
	if _, e := rand.Read(secret[:]); e != nil {
		return nil, e
	}
	token := hex.EncodeToString(secret[:])
	r, e := u.DB.One(c, "DesktopEngineCreate", id, arena, hash(token), actor)
	if e == nil {
		r["token"] = token
	}
	return r, e
}
func (u *UseCase) Revoke(c context.Context, actor, id string) error {
	if !core.UUID(id) {
		return apperror.ErrInvalid
	}
	r, e := u.DB.One(c, "DesktopEngineGet", id)
	if e != nil {
		return e
	}
	if e = u.ACL.Require(c, actor, text(r, "arena_id"), "ADMIN"); e != nil {
		return e
	}
	return u.DB.Exec(c, "DesktopEngineRevoke", id)
}
func (u *UseCase) Authenticate(c context.Context, token string) (core.Record, error) {
	if len(token) != 64 {
		return nil, apperror.ErrUnauthorized
	}
	r, e := u.DB.One(c, "DesktopEngineAuth", hash(token))
	if errors.Is(e, apperror.ErrNotFound) {
		e = apperror.ErrUnauthorized
	}
	return r, e
}
func (u *UseCase) Ingest(c context.Context, actor string, in model.Ingestion) (out core.Record, err error) {
	if !core.UUID(in.EngineID) || !core.UUID(in.RoundID) || !identifier.MatchString(in.SessionID) || len(in.Mappings) < 1 || len(in.Mappings) > 24 {
		return nil, apperror.ErrInvalid
	}
	slices.SortFunc(in.Mappings, func(a, b model.Mapping) int {
		if a.ChannelID < b.ChannelID {
			return -1
		}
		if a.ChannelID > b.ChannelID {
			return 1
		}
		return 0
	})
	seen := map[string]bool{}
	regs := map[string]bool{}
	for _, m := range in.Mappings {
		if !identifier.MatchString(m.ChannelID) || !core.UUID(m.RegistrationID) || !core.UUID(m.GantanganID) || seen[m.ChannelID] || regs[m.RegistrationID] {
			return nil, apperror.ErrInvalid
		}
		seen[m.ChannelID] = true
		regs[m.RegistrationID] = true
	}
	err = u.DB.Do(c, func(c context.Context) error {
		engine, e := u.DB.One(c, "DesktopEngineGet", in.EngineID)
		if e != nil {
			return e
		}
		arena := text(engine, "arena_id")
		if e = u.ACL.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
			return e
		}
		existing, e := u.DB.One(c, "DesktopSessionGet", in.EngineID, in.SessionID)
		if e == nil {
			if text(existing, "round_id") != in.RoundID {
				return apperror.ErrConflict
			}
			rows, e := u.DB.Many(c, "DesktopMappingList", text(existing, "id"))
			if e != nil {
				return e
			}
			if len(rows) != len(in.Mappings) {
				return apperror.ErrConflict
			}
			for i, r := range rows {
				m := in.Mappings[i]
				if text(r, "channel_id") != m.ChannelID || text(r, "registration_id") != m.RegistrationID || text(r, "gantangan_id") != m.GantanganID {
					return apperror.ErrConflict
				}
			}
			existing["mappings"] = rows
			out = existing
			return nil
		}
		if !errors.Is(e, apperror.ErrNotFound) {
			return e
		}
		id, e := core.NewID()
		if e != nil {
			return e
		}
		out, e = u.DB.One(c, "DesktopSessionCreate", id, in.EngineID, in.SessionID, in.RoundID, actor)
		if e != nil {
			return e
		}
		// Registration locks use UUID order to match concurrent mapping requests.
		ordered := slices.Clone(in.Mappings)
		slices.SortFunc(ordered, func(a, b model.Mapping) int {
			if a.RegistrationID < b.RegistrationID {
				return -1
			}
			if a.RegistrationID > b.RegistrationID {
				return 1
			}
			return 0
		})
		for _, m := range ordered {
			snap, e := u.DB.One(c, "DesktopMappingValidate", m.RegistrationID, arena, in.RoundID, m.GantanganID)
			if e != nil {
				return e
			}
			b, _ := json.Marshal(snap)
			if e = u.DB.Exec(c, "DesktopMappingCreate", id, m.ChannelID, m.RegistrationID, m.GantanganID, string(b)); e != nil {
				return e
			}
		}
		out["mappings"] = in.Mappings
		return nil
	})
	return
}
func (u *UseCase) Create(c context.Context, engine core.Record, in model.Upload) (out core.Record, err error) {
	if in.Container == "" {
		in.Container = "mp4"
	} // Existing clients/recordings.
	if in.Container != "mkv" && in.Container != "mp4" {
		return nil, apperror.ErrInvalid
	}
	if !identifier.MatchString(in.SessionID) || !identifier.MatchString(in.ChannelID) || !identifier.MatchString(in.RecordingID) || !digest.MatchString(in.SHA256) || in.Size <= 0 || in.Size > model.PartSize*10000 || in.Started.IsZero() || in.Ended.Before(in.Started) {
		return nil, apperror.ErrInvalid
	}
	if u.Objects == nil {
		return nil, apperror.ErrUnavailable
	}
	eid := text(engine, "id")
	err = u.DB.Do(c, func(c context.Context) error {
		s, e := u.DB.One(c, "DesktopSessionGet", eid, in.SessionID)
		if e != nil {
			return e
		}
		maps, e := u.DB.Many(c, "DesktopMappingList", text(s, "id"))
		if e != nil {
			return e
		}
		found := false
		for _, m := range maps {
			found = found || text(m, "channel_id") == in.ChannelID
		}
		if !found {
			return apperror.ErrForbidden
		}
		id, e := core.NewID()
		if e != nil {
			return e
		}
		key := fmt.Sprintf("arenas/%s/engines/%s/%s.%s", text(engine, "arena_id"), eid, id, in.Container)
		r, e := u.DB.One(c, "DesktopAssetCreate", id, eid, text(s, "id"), in.ChannelID, in.RecordingID, key, in.Size, in.SHA256, in.Started, in.Ended)
		if e != nil {
			return e
		}
		if !strings.HasSuffix(text(r, "object_key"), "."+in.Container) || text(r, "ingestion_id") != text(s, "id") || text(r, "channel_id") != in.ChannelID || text(r, "sha256") != in.SHA256 || fmt.Sprint(r["size_bytes"]) != strconv.FormatInt(in.Size, 10) {
			return apperror.ErrConflict
		}
		out, e = u.status(c, eid, text(r, "id"))
		return e
	})
	return
}
func (u *UseCase) status(c context.Context, eid, id string) (core.Record, error) {
	r, e := u.DB.One(c, "DesktopAssetLock", id, eid)
	if e != nil {
		return nil, e
	}
	if u.Objects == nil {
		return nil, apperror.ErrUnavailable
	}
	r["part_size"] = model.PartSize
	if text(r, "state") == "UPLOADED" {
		return r, nil
	}
	size, _ := strconv.ParseInt(fmt.Sprint(r["size_bytes"]), 10, 64)
	exists, e := u.Objects.Exists(c, text(r, "object_key"), size, text(r, "sha256"))
	if e != nil {
		return nil, e
	}
	if exists {
		return u.DB.One(c, "DesktopAssetComplete", id)
	}
	if text(r, "multipart_id") != "" {
		p, e := u.Objects.Parts(c, text(r, "object_key"), text(r, "multipart_id"))
		if e == nil {
			b, _ := json.Marshal(p)
			r, e = u.DB.One(c, "DesktopAssetParts", id, string(b))
			if e == nil {
				r["part_size"] = model.PartSize
			}
			return r, e
		}
		if !errors.Is(e, apperror.ErrNotFound) {
			return nil, e
		}
	}
	upload, e := u.Objects.Begin(c, text(r, "object_key"), text(r, "sha256"))
	if e != nil {
		return nil, e
	}
	r, e = u.DB.One(c, "DesktopAssetBegin", id, upload)
	if e == nil {
		r["part_size"] = model.PartSize
	}
	return r, e
}
func (u *UseCase) Status(c context.Context, engine core.Record, id string) (out core.Record, err error) {
	if !core.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	err = u.DB.Do(c, func(c context.Context) error { var e error; out, e = u.status(c, text(engine, "id"), id); return e })
	return
}
func (u *UseCase) Sign(c context.Context, engine core.Record, id string, p model.SignPart) (out model.Signed, err error) {
	if !core.UUID(id) || p.Number < 1 || p.Number > 10000 {
		return out, apperror.ErrInvalid
	}
	err = u.DB.Do(c, func(c context.Context) error {
		r, e := u.status(c, text(engine, "id"), id)
		if e != nil {
			return e
		}
		if text(r, "state") == "UPLOADED" {
			return apperror.ErrConflict
		}
		size, _ := strconv.ParseInt(fmt.Sprint(r["size_bytes"]), 10, 64)
		if int64(p.Number) > (size+model.PartSize-1)/model.PartSize {
			return apperror.ErrInvalid
		}
		out, e = u.Objects.Sign(c, text(r, "object_key"), text(r, "multipart_id"), p)
		return e
	})
	return
}
func (u *UseCase) Complete(c context.Context, engine core.Record, id string) (out core.Record, err error) {
	if !core.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	err = u.DB.Do(c, func(c context.Context) error {
		r, e := u.status(c, text(engine, "id"), id)
		if e != nil {
			return e
		}
		if text(r, "state") == "UPLOADED" {
			out = r
			return nil
		}
		p := parts(r)
		size, _ := strconv.ParseInt(fmt.Sprint(r["size_bytes"]), 10, 64)
		if int64(len(p)) != (size+model.PartSize-1)/model.PartSize {
			return apperror.ErrConflict
		}
		for i, v := range p {
			if v.Number != int32(i+1) {
				return apperror.ErrConflict
			}
		}
		if e = u.Objects.Complete(c, text(r, "object_key"), text(r, "multipart_id"), p); e != nil {
			return e
		}
		ok, e := u.Objects.Exists(c, text(r, "object_key"), size, text(r, "sha256"))
		if e != nil {
			return e
		}
		if !ok {
			return apperror.ErrConflict
		}
		out, e = u.DB.One(c, "DesktopAssetComplete", id)
		return e
	})
	return
}
func (u *UseCase) Read(c context.Context, actor, id string) (core.Record, error) {
	if !core.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	r, e := u.DB.One(c, "DesktopAssetRead", id)
	if e != nil {
		return nil, e
	}
	if text(r, "user_id") != actor {
		if e = u.ACL.Require(c, actor, text(r, "arena_id"), "ADMIN", "OPERATOR"); e != nil {
			return nil, e
		}
	}
	delete(r, "multipart_id")
	return r, nil
}
func (u *UseCase) List(c context.Context, actor, arena, round, session, channel, cursor string, limit int) ([]core.Record, error) {
	if !core.UUID(arena) || (round != "" && !core.UUID(round)) || (cursor != "" && !core.UUID(cursor)) || limit < 1 || limit > 101 {
		return nil, apperror.ErrInvalid
	}
	var own any
	if e := u.ACL.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
		if !errors.Is(e, apperror.ErrForbidden) {
			return nil, e
		}
		own = actor
	}
	return u.DB.Many(c, "DesktopAssetList", arena, null(round), null(session), null(channel), null(cursor), limit, own)
}
func (u *UseCase) Download(c context.Context, actor, id string) (string, error) {
	r, e := u.Read(c, actor, id)
	if e != nil {
		return "", e
	}
	if text(r, "state") != "UPLOADED" {
		return "", apperror.ErrConflict
	}
	if u.Objects == nil {
		return "", apperror.ErrUnavailable
	}
	return u.Objects.Download(c, text(r, "object_key"))
}
