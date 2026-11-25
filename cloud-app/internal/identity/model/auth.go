package model

import (
	"time"
)

type LoginRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	DeviceID  string `json:"device_id"`
	IP        string `json:"-"`
	UserAgent string `json:"-"`
}

type AccessibleScope struct {
	ScopeID   string  `json:"scope_id"`
	Role      string  `json:"role"`
	ScopeType string  `json:"scope_type"`
	ArenaID   *string `json:"arena_id,omitempty"`
}

type PreAuthResponse struct {
	PreAuthToken     string            `json:"pre_auth_token"`
	TokenType        string            `json:"token_type"`
	ExpiresIn        int               `json:"expires_in"`
	User             *UserProfile      `json:"user"`
	AccessibleScopes []AccessibleScope `json:"accessible_scopes"`
}

type SelectScopeRequest struct {
	PreAuthToken string `json:"pre_auth_token"`
	ScopeID      string `json:"scope_id"`
	DeviceID     string `json:"device_id"`
	IP           string `json:"-"`
	UserAgent    string `json:"-"`
}

type SwitchScopeRequest struct {
	ScopeID string `json:"scope_id"`
}

type AuthTokens struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	TokenType    string           `json:"token_type"`
	ExpiresIn    int              `json:"expires_in"`
	ActiveScope  *AccessibleScope `json:"active_scope,omitempty"`
	User         *UserProfile     `json:"user"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type SessionView struct {
	ID        string     `db:"id" json:"id"`
	UserID    string     `db:"user_id" json:"user_id"`
	DeviceID  string     `db:"device_id" json:"device_id"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	UserAgent string     `db:"user_agent" json:"user_agent"`
	IsCurrent bool       `json:"is_current"`
}

type SessionToken struct {
	UserID        string  `json:"user_id"`
	SessionID     string  `json:"session_id"`
	TokenType     string  `json:"token_type,omitempty"`
	ActiveRole    string  `json:"active_role,omitempty"`
	ActiveArenaID *string `json:"active_arena_id,omitempty"`
	Access        string  `json:"access"`
	Refresh       string  `json:"refresh"`
}
