package usecase

import (
	"context"
	"fmt"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	"strings"
	"time"
)

func merge(old, patch model.Record) model.Record {
	r := model.Record{}
	for k, v := range old {
		r[k] = v
	}
	for k, v := range patch {
		r[k] = v
	}
	return r
}
func (u *UseCase) activeUser(c context.Context, id string) (model.Record, error) {
	r, e := u.db.One(c, "MasterUserGet", id)
	if e != nil {
		return nil, e
	}
	if !model.Flag(r, "is_active") {
		return nil, apperror.ErrConflict
	}
	return r, nil
}
func (u *UseCase) Save(c context.Context, actor, res, id, arena string, f model.Record) (model.Record, error) {
	if res == "criteria" {
		return u.Criteria(c, actor, id, arena, f)
	}
	create := id == ""
	if !create {
		if e := validID(id); e != nil {
			return nil, e
		}
	}
	if res == "arenas" && !create {
		arena = id
	}
	if e := u.scoped(c, actor, res, arena, true); e != nil {
		return nil, e
	}
	allowed, required, ids, pos := "", "", "", ""
	switch res {
	case "users":
		allowed = "name phone email is_active is_eo expected_version nickname address position"
		required = "name phone"
	case "birds":
		allowed = "name owner_id bird_species_id expected_version"
		required = "name owner_id bird_species_id"
		ids = "owner_id bird_species_id"
	case "bird-species":
		allowed = "name"
		required = "name"
	case "arenas":
		allowed = "name address timezone is_active"
		required = "name"
	case "gantangan":
		allowed = "number description baris kolom"
		required = "number baris kolom"
		pos = required
	case "events":
		allowed = "owner_id title description event_date status location grand_prize pic_name min_participants max_participants"
		required = "owner_id title event_date"
		ids = "owner_id"
	case "categories":
		allowed = "code name"
		required = allowed
	case "rounds":
		allowed = "event_id category_id bird_species_id round_number title start_time end_time coordinator_name head_judge_name field_judge_1 field_judge_2 field_judge_3 field_judge_4 monitor_judge_1 monitor_judge_2 monitor_judge_3 monitor_judge_4"
		required = "event_id bird_species_id round_number title"
		ids = "event_id category_id bird_species_id"
		pos = "round_number"
	default:
		return nil, apperror.ErrNotFound
	}
	if res == "birds" && !create {
		allowed = "name bird_species_id expected_version"
		required = "name bird_species_id"
	}
	if e := validPatch(f, allowed, required, ids, pos, create); e != nil {
		return nil, e
	}
	for _, k := range []string{"name", "title", "code", "description", "timezone", "phone", "email", "event_date", "nickname", "address", "position", "location", "grand_prize", "pic_name", "start_time", "end_time", "coordinator_name", "head_judge_name", "field_judge_1", "field_judge_2", "field_judge_3", "field_judge_4", "monitor_judge_1", "monitor_judge_2", "monitor_judge_3", "monitor_judge_4"} {
		if v, ok := f[k]; ok && v != nil {
			if _, ok = v.(string); !ok {
				return nil, apperror.ErrInvalid
			}
		}
	}
	for _, k := range []string{"is_eo", "is_active"} {
		if v, ok := f[k]; ok {
			if _, ok = v.(bool); !ok {
				return nil, apperror.ErrInvalid
			}
		}
	}
	if e := email(f["email"]); e != nil {
		return nil, e
	}
	var out model.Record
	e := u.db.Do(c, func(c context.Context) error {
		if e := u.context(c, actor, arena); e != nil {
			return e
		}
		if arena != "" {
			if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
				return e
			}
		}
		var old model.Record
		var e error
		if !create {
			old, e = u.Get(c, actor, res, id, arena)
			if e != nil {
				return e
			}
		}
		r := merge(old, f)
		if res == "users" && !create {
			if id == actor && f["is_active"] == false {
				return apperror.Public(apperror.ErrConflict, "Akun sendiri tidak dapat dinonaktifkan")
			}
			if e = u.manageAccount(c, actor, id); e != nil {
				return e
			}
			if v, ok := f["is_active"]; ok && v == false {
				busy, e := u.db.One(c, "UserHasOpenJudging", id, nil)
				if e != nil {
					return e
				}
				if model.Flag(busy, "assigned") {
					return apperror.Public(apperror.ErrConflict, "Juri masih memiliki ronde berjalan")
				}
				if e = u.lastAdmin(c, id); e != nil {
					return e
				}
				if e = u.db.Exec(c, "LoginSessionRevoke", id, nil); e != nil {
					return e
				}
			}
			if _, e = u.version(c, "MasterUserLock", id, num(f, "expected_version")); e != nil {
				return e
			}
		}
		if res == "birds" {
			if !create {
				if _, e = u.version(c, "BirdLock", id, num(f, "expected_version")); e != nil {
					return e
				}
				if species, ok := f["bird_species_id"]; ok && species != old["bird_species_id"] {
					used, e := u.db.One(c, "BirdHasRegistrations", id)
					if e != nil {
						return e
					}
					if model.Flag(used, "used") {
						return apperror.Public(apperror.ErrConflict, "Jenis burung yang sudah digunakan pada tiket tidak dapat diubah")
					}
				}
			}
			if _, e = u.activeUser(c, str(r, "owner_id")); e != nil {
				return e
			}
		}
		if res == "rounds" && !create {
			locked, e := u.db.One(c, "RoundLock", arena, id)
			if e != nil {
				return e
			}
			if str(locked, "status") != "PREPARED" {
				return apperror.ErrConflict
			}
			tickets, e := u.db.Many(c, "RoundRegistrations", arena, id)
			if e != nil {
				return e
			}
			if len(tickets) > 0 {
				for _, k := range []string{"event_id", "bird_species_id", "category_id"} {
					if v, ok := f[k]; ok && fmt.Sprint(v) != fmt.Sprint(old[k]) {
						return apperror.ErrConflict
					}
				}
			}
		}
		switch res {
		case "users":
			if create {
				id, e = model.NewID()
				if e != nil {
					return e
				}
				eo, _ := f["is_eo"].(bool)
				active := true
				if v, ok := f["is_active"].(bool); ok {
					active = v
				}
				out, e = u.db.One(c, "AccountCreate", id, str(f, "name"), null(str(f, "phone")), null(str(f, "email")), nil, eo, active, null(str(f, "nickname")), null(str(f, "address")), null(str(f, "position")))
				if e == nil {
					e = u.db.Exec(c, "ParticipantAssignRole", id)
				}
			} else {
				patch := model.Record{}
				for k, v := range f {
					if k != "expected_version" {
						patch[k] = v
					}
				}
				out, e = u.db.One(c, "MasterUserUpdate", id, num(f, "expected_version"), jsonValue(patch))
				_, changedEmail := f["email"]
				if e == nil && changedEmail && f["email"] != old["email"] {
					e = u.db.Exec(c, "ResetEmailVerification", id)
					if e == nil {
						e = u.db.Exec(c, "LoginSessionRevoke", id, nil)
					}
				}
			}
		case "birds":
			if create {
				out, e = u.db.One(c, "BirdCreate", str(f, "name"), str(f, "owner_id"), str(f, "bird_species_id"))
			} else {
				patch := model.Record{}
				for k, v := range f {
					if k != "expected_version" {
						patch[k] = v
					}
				}
				out, e = u.db.One(c, "BirdUpdate", id, num(f, "expected_version"), jsonValue(patch))
			}
		case "arenas":
			if create {
				zone := str(f, "timezone")
				if zone == "" {
					zone = "Asia/Jakarta"
				}
				if _, e = time.LoadLocation(zone); e != nil {
					return apperror.Invalid("Zona waktu tidak valid")
				}
				active := true
				if v, ok := f["is_active"].(bool); ok {
					active = v
				}
				out, e = u.db.One(c, "ArenaCreate", str(f, "name"), null(str(f, "address")), zone, active)
				if e == nil {
					id = str(out, "id")
					e = u.db.Exec(c, "AssignArenaRole", actor, id, "ADMIN")
				}
			} else {
				if v, ok := f["timezone"]; ok {
					if _, e = time.LoadLocation(fmt.Sprint(v)); e != nil {
						return apperror.ErrInvalid
					}
				}
				out, e = u.db.One(c, "ArenaUpdate", id, jsonValue(f))
			}
		case "bird-species":
			if create {
				out, e = u.db.One(c, "BirdSpeciesCreate", str(f, "name"))
			} else {
				out, e = u.db.One(c, "BirdSpeciesUpdate", id, jsonValue(f))
			}
		case "gantangan":
			if create {
				out, e = u.db.One(c, "GantanganCreate", arena, num(f, "number"), null(str(f, "description")), num(f, "baris"), num(f, "kolom"))
			} else {
				out, e = u.db.One(c, "GantanganUpdate", arena, id, jsonValue(f))
			}
		case "events":
			owner, e := u.activeUser(c, str(r, "owner_id"))
			if e != nil {
				return e
			}
			if !model.Flag(owner, "is_eo") {
				return apperror.Invalid("Penyelenggara harus akun EO")
			}
			if _, e = time.Parse("2006-01-02", str(r, "event_date")); e != nil {
				return apperror.ErrInvalid
			}
			status := str(r, "status")
			if status == "" {
				status = "SCHEDULED"
			}
			if !strings.Contains("|SCHEDULED|ACTIVE|COMPLETED|CANCELLED|", "|"+status+"|") {
				return apperror.ErrInvalid
			}
			if create {
				out, e = u.db.One(c, "EventCreate", arena, str(r, "owner_id"), str(r, "title"), null(str(r, "description")), str(r, "event_date"), status, null(str(r, "location")), null(str(r, "grand_prize")), null(str(r, "pic_name")), num(r, "min_participants"), num(r, "max_participants"))
			} else {
				out, e = u.db.One(c, "EventUpdate", arena, id, jsonValue(f))
			}
		case "categories":
			if create {
				out, e = u.db.One(c, "CategoryCreate", arena, str(f, "code"), str(f, "name"))
			} else {
				out, e = u.db.One(c, "CategoryUpdate", arena, id, jsonValue(f))
			}
		case "rounds":
			if cat := str(r, "category_id"); cat != "" {
				if _, e = u.db.One(c, "CategoryGet", arena, cat); e != nil {
					return e
				}
			}
			if _, e = u.db.One(c, "EventGet", arena, str(r, "event_id")); e != nil {
				return e
			}
			if create {
				out, e = u.db.One(c, "RoundCreate", arena, str(f, "event_id"), null(str(f, "category_id")), str(f, "bird_species_id"), num(f, "round_number"), str(f, "title"), null(str(f, "start_time")), null(str(f, "end_time")), null(str(f, "coordinator_name")), null(str(f, "head_judge_name")), null(str(f, "field_judge_1")), null(str(f, "field_judge_2")), null(str(f, "field_judge_3")), null(str(f, "field_judge_4")), null(str(f, "monitor_judge_1")), null(str(f, "monitor_judge_2")), null(str(f, "monitor_judge_3")), null(str(f, "monitor_judge_4")))
			} else {
				out, e = u.db.One(c, "RoundUpdate", arena, id, jsonValue(f))
			}
		}
		if e != nil {
			return e
		}
		if id == "" {
			id = str(out, "id")
		}
		out, e = u.Get(c, actor, res, id, arena)
		return e
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
func (u *UseCase) Delete(c context.Context, actor, res, id, arena string, version int) error {
	if e := validID(id); e != nil {
		return e
	}
	if res == "arenas" {
		arena = id
	}
	if e := u.scoped(c, actor, res, arena, true); e != nil {
		return e
	}
	return u.db.Do(c, func(c context.Context) error {
		if arena != "" {
			if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
				return e
			}
		}
		if e := u.context(c, actor, arena); e != nil {
			return e
		}
		if res == "users" {
			busy, e := u.db.One(c, "UserHasOpenJudging", id, nil)
			if e != nil {
				return e
			}
			if model.Flag(busy, "assigned") {
				return apperror.Public(apperror.ErrConflict, "Juri masih memiliki ronde berjalan")
			}
			if id == actor {
				return apperror.Public(apperror.ErrConflict, "Akun sendiri tidak dapat diarsipkan")
			}
			if e := u.manageAccount(c, actor, id); e != nil {
				return e
			}
			if e := u.lastAdmin(c, id); e != nil {
				return e
			}
			if _, e := u.version(c, "MasterUserLock", id, version); e != nil {
				return e
			}
			_, e = u.db.One(c, "MasterUserSoftDelete", id, version)
			return e
		}
		if res == "birds" {
			if _, e := u.version(c, "BirdLock", id, version); e != nil {
				return e
			}
			_, e := u.db.One(c, "BirdSoftDelete", id, version)
			return e
		}
		if res == "rounds" {
			r, e := u.db.One(c, "RoundLock", arena, id)
			if e != nil {
				return e
			}
			if str(r, "status") != "PREPARED" {
				return apperror.ErrConflict
			}
		}
		if res == "criteria" {
			return u.criteriaDelete(c, actor, id, arena)
		}
		if res == "arenas" {
			if e := u.db.Exec(c, "ClearArenaAssignments", arena); e != nil {
				return e
			}
		}
		p := map[string]string{"arenas": "Arena", "bird-species": "BirdSpecies", "gantangan": "Gantangan", "events": "Event", "categories": "Category", "rounds": "Round"}[res]
		if p == "" {
			return apperror.ErrNotFound
		}
		if res == "arenas" || res == "bird-species" {
			_, e := u.db.One(c, p+"Delete", id)
			return e
		}
		_, e := u.db.One(c, p+"Delete", arena, id)
		return e
	})
}
func (u *UseCase) History(c context.Context, actor, res, id string, after, limit int) ([]model.Record, error) {
	if e := u.Staff(c, actor); e != nil {
		return nil, e
	}
	if !model.UUID(id) || after < 0 || limit < 1 || limit > 100 {
		return nil, apperror.ErrInvalid
	}
	q := map[string]string{"users": "MasterUserHistory", "birds": "BirdMasterHistory"}[res]
	if q == "" {
		return nil, apperror.ErrNotFound
	}
	return u.db.Many(c, q, id, after, limit+1)
}
func (u *UseCase) Transfer(c context.Context, actor, id, owner, notes string, version int) (model.Record, error) {
	if e := u.Staff(c, actor); e != nil {
		return nil, e
	}
	if !model.UUID(id) || !model.UUID(owner) || len(notes) > 2000 {
		return nil, apperror.ErrInvalid
	}
	e := u.db.Do(c, func(c context.Context) error {
		if e := u.context(c, actor, ""); e != nil {
			return e
		}
		if _, e := u.activeUser(c, owner); e != nil {
			return e
		}
		if _, e := u.version(c, "BirdLock", id, version); e != nil {
			return e
		}
		_, e := u.db.One(c, "BirdTransferOwner", id, version, owner, null(notes))
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.Get(c, actor, "birds", id, "")
}
func (u *UseCase) Ownership(c context.Context, actor, id string) ([]model.Record, error) {
	if e := u.Staff(c, actor); e != nil {
		return nil, e
	}
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	return u.db.Many(c, "BirdOwnershipHistory", id)
}
