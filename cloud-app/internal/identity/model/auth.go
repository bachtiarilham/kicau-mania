package model

import (
	"time"
)

// LoginRequest describes incoming user credentials and device info.
type LoginRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	DeviceID  string `json:"device_id"`
	IP        string `json:"-"`
	UserAgent string `json:"-"`
}

// AuthTokens contains JWT tokens and profile data returned on login/refresh.
type AuthTokens struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int          `json:"expires_in"`
	User         *UserProfile `json:"user"`
}

// ChangePasswordRequest describes payload for password alteration.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// SessionView represents a row projected from cloud_read.v_user_sessions.
type SessionView struct {
	ID        string     `db:"id" json:"id"`
	UserID    string     `db:"user_id" json:"user_id"`
	DeviceID  string     `db:"device_id" json:"device_id"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	UserAgent string     `db:"user_agent" json:"user_agent"`
	IsCurrent bool       `json:"is_current"`
}

// SessionToken represents active token session data in Redis.
type SessionToken struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
}
