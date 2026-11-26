package app_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	competition "kicau.cloud/control/internal/competition/repository"
	core "kicau.cloud/control/internal/core/model"
	identity "kicau.cloud/control/internal/identity/repository"
	"kicau.cloud/control/internal/infra/postgres"
)

func TestMigratedNumericIDs(t *testing.T) {
	url := os.Getenv("NUMERIC_ID_TEST_URL")
	if url == "" {
		t.Skip("requires an isolated migrated development fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := &postgres.Store{Pool: pool}
	userRepo := identity.NewUserRepository(store)
	var uid, arena, ticket string
	err = pool.QueryRow(ctx, "select user_id::text,arena_id::text,id::text from competition.registrations limit 1").Scan(&uid, &arena, &ticket)
	if err != nil {
		t.Fatal(err)
	}
	user, err := userRepo.GetProfile(ctx, uid)
	if err != nil || user.ID != uid {
		t.Fatalf("user lookup: %v", err)
	}
	reg, err := competition.NewRegistrationRepository(store).GetByID(ctx, arena, ticket)
	if err != nil || reg.ID != ticket {
		t.Fatalf("ticket lookup: %v", err)
	}
	fresh, err := userRepo.NextID(ctx)
	if err != nil || !core.NumericID(fresh) {
		t.Fatalf("user sequence: %v", err)
	}
}
