// Package model defines authenticated identities independently of HTTP and storage.
package model

type UserIdentity struct{ UserID string }
type EngineIdentity struct{ ArenaID, EngineID string }
