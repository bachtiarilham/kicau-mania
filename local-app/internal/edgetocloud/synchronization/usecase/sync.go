// Package usecase delivers the durable arena outbox without participating in scoring.
package usecase

import (
	"context"
	"fmt"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
)

type Repository interface {
	Pending(context.Context, int) ([]exchange.Change, error)
	Acknowledge(context.Context, []string) error
}
type Cloud interface {
	Send(context.Context, []exchange.Change) ([]string, error)
}
type UseCase struct {
	repo  Repository
	cloud Cloud
}

func New(repo Repository, cloud Cloud) *UseCase { return &UseCase{repo: repo, cloud: cloud} }
func (u *UseCase) Flush(ctx context.Context) error {
	if u.cloud == nil {
		return nil
	}
	batch, err := u.repo.Pending(ctx, 100)
	if err != nil {
		return err
	}
	if len(batch) == 0 {
		return nil
	}
	ids, err := u.cloud.Send(ctx, batch)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, v := range batch {
		allowed[v.ID] = true
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !allowed[id] || seen[id] {
			return fmt.Errorf("invalid cloud acknowledgment")
		}
		seen[id] = true
	}
	// Unacknowledged changes remain pending, including on timeout or malformed replies.
	return u.repo.Acknowledge(ctx, ids)
}
