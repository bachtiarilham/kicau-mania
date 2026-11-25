package redis

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"kicau.cloud/control/internal/identity/model"
)

type sessionKeys map[string]string

func (s sessionKeys) DialHook(next goredis.DialHook) goredis.DialHook { return next }
func (s sessionKeys) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}
func (s sessionKeys) ProcessHook(_ goredis.ProcessHook) goredis.ProcessHook {
	return func(_ context.Context, cmd goredis.Cmder) error {
		args := cmd.Args()
		switch c := cmd.(type) {
		case *goredis.Cmd:
			// EVAL arguments contain the three keys and the serialized session.
			for _, key := range args[3:6] {
				s[key.(string)] = args[7].(string)
			}
			c.SetVal(int64(1))
		case *goredis.StringCmd:
			value, ok := s[args[1].(string)]
			if !ok {
				return goredis.Nil
			}
			c.SetVal(value)
		}
		return nil
	}
}

func TestIssuedAccessAndRefreshTokensResolve(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:6379"})
	defer client.Close()
	client.AddHook(sessionKeys{})
	store := &Store{Client: client}
	ctx := context.Background()
	want := model.SessionToken{UserID: "user", SessionID: "session", Access: "access-digest", Refresh: "refresh-digest"}
	if err := store.Rotate(ctx, "", want, time.Minute, time.Hour); err != nil {
		t.Fatal(err)
	}
	for kind, digest := range map[string]string{"access": want.Access, "refresh": want.Refresh} {
		got, err := store.Resolve(ctx, kind, digest)
		if err != nil || got != want {
			t.Fatalf("%s token did not resolve to the issued session: %v", kind, err)
		}
	}
}
