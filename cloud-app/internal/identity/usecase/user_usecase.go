package usecase

import (
	"context"

	"net/mail"
	"strings"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/identity/contract"
	"kicau.cloud/control/internal/identity/model"
)

type UserUsecase struct {
	tx        contract.TransactionManager
	users     contract.UserRepository
	passwords PasswordHasher
}

func NewUserUsecase(tx contract.TransactionManager, users contract.UserRepository, passwords PasswordHasher) *UserUsecase {
	return &UserUsecase{tx: tx, users: users, passwords: passwords}
}

func (u *UserUsecase) Get(ctx context.Context, actorID, targetID string) (*model.UserView, error) {
	return u.users.GetByID(ctx, targetID)
}

func (u *UserUsecase) List(ctx context.Context, actorID string, filter model.UserFilter) ([]model.UserView, error) {
	return u.users.List(ctx, filter)
}

func (u *UserUsecase) Create(ctx context.Context, actorID string, req *model.CreateUserRequest) (*model.UserView, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.Invalid("Nama wajib diisi")
	}
	if req.Email != nil && *req.Email != "" {
		trimmed := strings.ToLower(strings.TrimSpace(*req.Email))
		if addr, err := mail.ParseAddress(trimmed); err != nil || addr.Address != trimmed {
			return nil, apperror.Invalid("Format email tidak valid")
		}
		req.Email = &trimmed
	}

	id, err := u.users.NextID(ctx)
	if err != nil {
		return nil, err
	}

	var hash *string
	if req.Password != nil && *req.Password != "" {
		h, err := u.passwords.Hash(ctx, *req.Password)
		if err != nil {
			return nil, err
		}
		hash = &h
	}

	var created *model.UserView
	err = u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.users.SetMutationContext(txCtx, actorID, nil); err != nil {
			return err
		}
		c, err := u.users.Create(txCtx, id, req, hash)
		if err != nil {
			return err
		}
		created = c
		return nil
	})

	return created, err
}

func (u *UserUsecase) Update(ctx context.Context, actorID, targetID string, req *model.UpdateUserRequest) (*model.UserView, error) {
	if req.ExpectedVersion < 1 {
		return nil, apperror.Invalid("expected_version wajib diisi")
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return nil, apperror.Invalid("Nama tidak boleh kosong")
	}
	if req.Email != nil && *req.Email != "" {
		trimmed := strings.ToLower(strings.TrimSpace(*req.Email))
		if addr, err := mail.ParseAddress(trimmed); err != nil || addr.Address != trimmed {
			return nil, apperror.Invalid("Format email tidak valid")
		}
		req.Email = &trimmed
	}

	var updated *model.UserView
	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.users.SetMutationContext(txCtx, actorID, nil); err != nil {
			return err
		}
		uView, err := u.users.Update(txCtx, targetID, req)
		if err != nil {
			return err
		}
		updated = uView
		return nil
	})

	return updated, err
}

func (u *UserUsecase) Delete(ctx context.Context, actorID, targetID string, expectedVersion int) error {
	if actorID == targetID {
		return apperror.Public(apperror.ErrConflict, "Akun sendiri tidak dapat dinonaktifkan")
	}
	return u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.users.SetMutationContext(txCtx, actorID, nil); err != nil {
			return err
		}
		return u.users.Delete(txCtx, targetID, expectedVersion)
	})
}

func (u *UserUsecase) Require(ctx context.Context, actorID, arenaID string, roles ...string) error {
	profile, err := u.users.GetProfile(ctx, actorID)
	if err != nil {
		return err
	}
	for _, r := range profile.Roles {
		if r == "ADMIN" {
			return nil
		}
	}
	for _, ar := range profile.ArenaRoles {
		if ar.ArenaID == arenaID {
			for _, r := range roles {
				if ar.Role == r {
					return nil
				}
			}
		}
	}
	return apperror.ErrForbidden
}

func (u *UserUsecase) Bootstrap(ctx context.Context, name, emailAddress, phone, password string) error {
	if strings.TrimSpace(name) == "" || phone == "" || emailAddress == "" {
		return apperror.ErrInvalid
	}
	emailAddress = strings.ToLower(strings.TrimSpace(emailAddress))
	if addr, err := mail.ParseAddress(emailAddress); err != nil || addr.Address != emailAddress {
		return apperror.Invalid("Format email tidak valid")
	}
	hash, err := u.passwords.Hash(ctx, password)
	if err != nil {
		return err
	}
	id, err := u.users.NextID(ctx)
	if err != nil {
		return err
	}
	return u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.users.BootstrapLock(txCtx); err != nil {
			return err
		}
		exists, err := u.users.AdminExists(txCtx)
		if err != nil {
			return err
		}
		if exists {
			return apperror.Public(apperror.ErrConflict, "ADMIN pertama sudah tersedia")
		}
		if err := u.users.SetMutationContext(txCtx, id, nil); err != nil {
			return err
		}
		req := &model.CreateUserRequest{
			Name:  name,
			Email: &emailAddress,
			Phone: &phone,
		}
		created, err := u.users.Create(txCtx, id, req, &hash)
		if err != nil {
			return err
		}
		if err := u.users.VerifyEmail(txCtx, created.ID); err != nil {
			return err
		}
		return u.users.AssignGlobalRole(txCtx, created.ID, "ADMIN")
	})
}
