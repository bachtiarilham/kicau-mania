package model

type Session struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
}
