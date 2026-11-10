package model

import "kicau.cloud/control/internal/core/model"

type Login struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	DeviceID  string `json:"device_id"`
	UserAgent string `json:"-"`
	IP        string `json:"-"`
}
type Register struct {
	IP       string `json:"-"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}
type GoogleLogin struct {
	Credential string `json:"credential"`
	Phone      string `json:"phone"`
	DeviceID   string `json:"device_id"`
	UserAgent  string `json:"-"`
	IP         string `json:"-"`
}
type GoogleIdentity struct {
	Subject, Email, Name string
	Authoritative        bool
}
type Tokens struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"-"`
	ExpiresIn    int          `json:"expires_in"`
	User         model.Record `json:"user"`
}
