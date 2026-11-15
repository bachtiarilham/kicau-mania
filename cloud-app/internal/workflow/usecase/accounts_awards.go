package usecase

import (
	"context"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	"strings"
	"time"
)

func (u *UseCase) Assignments(c context.Context, actor, arena string) ([]model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN"); e != nil {
		return nil, e
	}
	return u.db.Many(c, "ActorAssignments", arena)
}
func (u *UseCase) AssignRole(c context.Context, actor, arena, user, role string, remove bool) ([]model.Record, error) {
	if e := u.Require(c, actor, arena, "ADMIN"); e != nil {
		return nil, e
	}
	if !model.UUID(user) || (role != "ADMIN" && role != "OPERATOR" && role != "JURI") {
		return nil, apperror.ErrInvalid
	}
	e := u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
			return e
		}
		if !remove {
			if _, e := u.activeUser(c, user); e != nil {
				return e
			}
		}
		q := "AssignArenaRole"
		if remove {
			if role == "JURI" {
				busy, e := u.db.One(c, "UserHasOpenJudging", user, arena)
				if e != nil {
					return e
				}
				if model.Flag(busy, "assigned") {
					return apperror.Public(apperror.ErrConflict, "Juri masih memiliki ronde berjalan")
				}
			}
			rows, e := u.db.Many(c, "ActorArenaRoles", user, arena)
			if e != nil {
				return e
			}
			if role == "ADMIN" && has(rows, "ADMIN") {
				r, e := u.db.One(c, "AssignmentCount", arena)
				if e != nil {
					return e
				}
				if num(r, "count") <= 1 {
					return apperror.Public(apperror.ErrConflict, "Admin terakhir tidak dapat dicabut")
				}
			}
			q = "RemoveArenaRole"
		}
		return u.db.Exec(c, q, user, arena, role)
	})
	if e != nil {
		return nil, e
	}
	return u.Assignments(c, actor, arena)
}
func (u *UseCase) Credentials(c context.Context, actor, arena, user, password string) error {
	if e := u.Require(c, actor, arena, "ADMIN"); e != nil {
		return e
	}
	if !model.UUID(user) {
		return apperror.ErrInvalid
	}
	if actor == user {
		return apperror.Invalid("Gunakan ganti kata sandi untuk akun sendiri")
	}
	hash, e := u.passwords.Hash(c, password)
	if e != nil {
		return e
	}
	return u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
			return e
		}
		target, e := u.activeUser(c, user)
		if e != nil {
			return e
		}
		if str(target, "email") == "" {
			return apperror.Invalid("Email wajib tersedia untuk login")
		}
		if model.Flag(target, "has_password") {
			return apperror.Public(apperror.ErrConflict, "Password awal sudah ditetapkan; gunakan ganti password pada akun terkait")
		}
		if e = u.manageAccount(c, actor, user); e != nil {
			return e
		}
		roles, e := u.db.Many(c, "ActorArenaRoles", user, arena)
		if e != nil {
			return e
		}
		if !has(roles, "ADMIN", "OPERATOR", "JURI") {
			return apperror.Invalid("Tugaskan role petugas di arena ini terlebih dahulu")
		}
		if _, e = u.db.One(c, "LoginUserLock", user); e != nil {
			return e
		}
		if _, e = u.db.One(c, "AccountSetPassword", user, hash); e != nil {
			return e
		}
		if _, e = u.db.One(c, "AccountVerifyEmail", user); e != nil {
			return e
		}
		return u.db.Exec(c, "LoginSessionRevoke", user, nil)
	})
}
func (u *UseCase) ChangePassword(c context.Context, user, old, password string) error {
	r, e := u.db.One(c, "AccountCredentialByID", user)
	if e != nil {
		return e
	}
	ok, e := u.passwords.Verify(c, str(r, "password_hash"), old)
	if e != nil {
		return e
	}
	if !ok {
		return apperror.ErrUnauthorized
	}
	hash, e := u.passwords.Hash(c, password)
	if e != nil {
		return e
	}
	return u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "LoginUserLock", user); e != nil {
			return e
		}
		current, e := u.db.One(c, "AccountCredentialByID", user)
		if e != nil {
			return e
		}
		if str(current, "password_hash") != str(r, "password_hash") {
			return apperror.ErrConflict
		}
		if _, e = u.db.One(c, "AccountSetPassword", user, hash); e != nil {
			return e
		}
		return u.db.Exec(c, "LoginSessionRevoke", user, nil)
	})
}
func (u *UseCase) Bootstrap(c context.Context, name, emailAddress, phone, password string) error {
	if strings.TrimSpace(name) == "" || phone == "" || emailAddress == "" {
		return apperror.ErrInvalid
	}
	if e := email(emailAddress); e != nil {
		return e
	}
	hash, e := u.passwords.Hash(c, password)
	if e != nil {
		return e
	}
	id, e := model.NewID()
	if e != nil {
		return e
	}
	return u.db.Do(c, func(c context.Context) error {
		if e := u.db.Exec(c, "BootstrapLock"); e != nil {
			return e
		}
		r, e := u.db.One(c, "BootstrapAdminExists")
		if e != nil {
			return e
		}
		if model.Flag(r, "exists") {
			return apperror.Public(apperror.ErrConflict, "ADMIN pertama sudah tersedia")
		}
		if e = u.context(c, id, ""); e != nil {
			return e
		}
		if _, e = u.db.One(c, "AccountCreate", id, name, phone, emailAddress, hash, false, true); e != nil {
			return e
		}
		if _, e = u.db.One(c, "AccountVerifyEmail", id); e != nil {
			return e
		}
		return u.db.Exec(c, "AssignGlobalRole", id, "ADMIN")
	})
}
func (u *UseCase) Awards(c context.Context, actor, bird, cursor string, limit int) ([]model.Record, error) {
	if _, e := u.Get(c, actor, "birds", bird, ""); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 || cursor != "" && !model.UUID(cursor) {
		return nil, apperror.ErrInvalid
	}
	return u.db.Many(c, "BirdAwardList", bird, null(cursor), limit+1)
}
func (u *UseCase) Award(c context.Context, actor, bird, id string) (model.Record, error) {
	if _, e := u.Get(c, actor, "birds", bird, ""); e != nil {
		return nil, e
	}
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	return u.db.One(c, "BirdAwardGet", bird, id)
}
func (u *UseCase) SaveAward(c context.Context, actor, arena, bird, id string, f model.Record) (model.Record, error) {
	if e := u.Staff(c, actor); e != nil {
		return nil, e
	}
	if !model.UUID(bird) || id != "" && !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	if e := validPatch(f, "registration_id award_name event_name organizer_name rank awarded_at certificate_url", "award_name awarded_at", "registration_id", "rank", true); e != nil {
		return nil, e
	}
	if e := urlValue(f["certificate_url"]); e != nil {
		return nil, e
	}
	if _, e := time.Parse("2006-01-02", str(f, "awarded_at")); e != nil {
		return nil, apperror.ErrInvalid
	}
	e := u.db.Do(c, func(c context.Context) error {
		reg := str(f, "registration_id")
		if id != "" {
			old, e := u.db.One(c, "AwardRaw", bird, id)
			if e != nil {
				return e
			}
			if str(old, "registration_id") != reg {
				return apperror.Invalid("Hubungan tiket penghargaan tidak dapat dipindah")
			}
			if _, present := f["certificate_url"]; !present {
				f["certificate_url"] = old["certificate_url"]
			}
		}
		if reg != "" {
			if e := u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
				return e
			}
			if _, e := u.db.One(c, "ArenaLock", arena); e != nil {
				return e
			}
			t, e := u.db.One(c, "RegistrationGet", arena, reg)
			if e != nil {
				return e
			}
			if _, e = u.db.One(c, "RoundLock", arena, str(t, "round_id")); e != nil {
				return e
			}
			if _, e = u.db.One(c, "RegistrationLock", arena, reg); e != nil {
				return e
			}
			if str(t, "bird_id") != bird || str(t, "participant_status") != "ACTIVE" || str(t, "status") != "CONFIRMED" || str(t, "payment_status") != "VERIFIED" {
				return apperror.ErrConflict
			}
		} else if strings.TrimSpace(str(f, "event_name")) == "" {
			return apperror.Invalid("Nama event eksternal wajib diisi")
		}
		b, e := u.db.One(c, "BirdLock", bird)
		if e != nil {
			return e
		}
		if b["deleted_at"] != nil {
			return apperror.ErrConflict
		}
		rank := any(nil)
		if f["rank"] != nil {
			rank = num(f, "rank")
		}
		var out model.Record
		if id == "" {
			out, e = u.db.One(c, "BirdAwardCreate", bird, null(reg), str(f, "award_name"), null(str(f, "event_name")), null(str(f, "organizer_name")), rank, str(f, "awarded_at"), null(str(f, "certificate_url")))
		} else {
			out, e = u.db.One(c, "BirdAwardUpdate", bird, id, str(f, "award_name"), null(str(f, "event_name")), null(str(f, "organizer_name")), rank, str(f, "awarded_at"), null(str(f, "certificate_url")))
		}
		if e == nil {
			id = str(out, "id")
		}
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.Award(c, actor, bird, id)
}
func (u *UseCase) DeleteAward(c context.Context, actor, arena, bird, id string) error {
	if e := u.Staff(c, actor); e != nil {
		return e
	}
	old, e := u.db.One(c, "AwardRaw", bird, id)
	if e != nil {
		return e
	}
	if str(old, "registration_id") != "" {
		if e = u.Require(c, actor, arena, "ADMIN", "OPERATOR"); e != nil {
			return e
		}
		if _, e = u.db.One(c, "RegistrationGet", arena, str(old, "registration_id")); e != nil {
			return e
		}
	}
	_, e = u.db.One(c, "BirdAwardDelete", bird, id)
	return e
}
func (u *UseCase) Certificate(c context.Context, actor, bird, id string) (string, error) {
	if _, e := u.Get(c, actor, "birds", bird, ""); e != nil {
		return "", e
	}
	r, e := u.db.One(c, "AwardRaw", bird, id)
	if e != nil {
		return "", e
	}
	link := str(r, "certificate_url")
	if link == "" {
		return "", apperror.ErrNotFound
	}
	if e = urlValue(link); e != nil {
		return "", e
	}
	return link, nil
}
