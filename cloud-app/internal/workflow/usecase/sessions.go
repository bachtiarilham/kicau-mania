package usecase

import (
	"context"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) Sessions(c context.Context, actor string) ([]model.Record, error) {
	return u.db.Many(c, "LoginSessionList", actor)
}
func (u *UseCase) RevokeSession(c context.Context, actor, id string) error {
	if !model.UUID(id) {
		return apperror.ErrInvalid
	}
	return u.db.Exec(c, "LoginSessionRevoke", actor, id)
}
