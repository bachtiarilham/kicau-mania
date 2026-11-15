// Package service stores only SHA-256 token digests, never bearer credentials.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/redis/go-redis/v9"
	"kicau.cloud/control/internal/apperror"
	input "kicau.cloud/control/internal/login/model"
	"time"
)

type Store struct{ Client *redis.Client }

func New(raw string) (*Store, error) {
	if raw == "" {
		return &Store{}, nil
	}
	o, e := redis.ParseURL(raw)
	if e != nil {
		return nil, apperror.ErrInvalid
	}
	o.DialTimeout = 3 * time.Second
	o.ReadTimeout = 3 * time.Second
	o.WriteTimeout = 3 * time.Second
	return &Store{redis.NewClient(o)}, nil
}
func (s *Store) Ping(c context.Context) error {
	if s.Client == nil {
		return apperror.ErrUnavailable
	}
	if s.Client.Ping(c).Err() != nil {
		return apperror.ErrUnavailable
	}
	return nil
}
func (s *Store) Close() {
	if s.Client != nil {
		_ = s.Client.Close()
	}
}
func (s *Store) Resolve(c context.Context, kind, hash string) (input.Session, error) {
	var out input.Session
	if s.Client == nil {
		return out, apperror.ErrUnavailable
	}
	v, e := s.Client.Get(c, "km:"+kind+":"+hash).Result()
	if errors.Is(e, redis.Nil) {
		return out, apperror.ErrUnauthorized
	}
	if e != nil {
		return out, apperror.ErrUnavailable
	}
	if json.Unmarshal([]byte(v), &out) != nil {
		return out, apperror.ErrUnavailable
	}
	return out, nil
}

// Lua serializes refresh rotation and rejects concurrent reuse of an old digest.
const rotate = `
local old=redis.call('GET',KEYS[1])
if ARGV[1]~='' then
 if not old then return 0 end
 local v=cjson.decode(old)
 if v.refresh~=ARGV[1] then return 0 end
end
if old then
 local v=cjson.decode(old)
 redis.call('DEL','km:a:'..v.access,'km:r:'..v.refresh)
end
redis.call('SET',KEYS[1],ARGV[2],'PX',ARGV[4])
redis.call('SET',KEYS[2],ARGV[2],'PX',ARGV[3])
redis.call('SET',KEYS[3],ARGV[2],'PX',ARGV[4])
return 1`

func (s *Store) Rotate(c context.Context, old string, v input.Session, accessTTL, refreshTTL time.Duration) error {
	if s.Client == nil {
		return apperror.ErrUnavailable
	}
	body, _ := json.Marshal(v)
	n, e := s.Client.Eval(c, rotate, []string{"km:s:" + v.SessionID, "km:a:" + v.Access, "km:r:" + v.Refresh}, old, string(body), accessTTL.Milliseconds(), refreshTTL.Milliseconds()).Int()
	if e != nil {
		return apperror.ErrUnavailable
	}
	if n != 1 {
		return apperror.ErrUnauthorized
	}
	return nil
}
func (s *Store) Delete(c context.Context, v input.Session) error {
	if s.Client == nil {
		return apperror.ErrUnavailable
	}
	e := s.Client.Del(c, "km:s:"+v.SessionID, "km:a:"+v.Access, "km:r:"+v.Refresh).Err()
	if e != nil {
		return apperror.ErrUnavailable
	}
	return nil
}
