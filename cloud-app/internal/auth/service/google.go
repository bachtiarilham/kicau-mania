package service

import (
	"context"
	"strings"

	"cloud.google.com/go/auth/credentials/idtoken"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/auth/model"
)

type Google struct{ ClientID string }

func (g Google) Verify(ctx context.Context, credential string) (model.GoogleIdentity, error) {
	var out model.GoogleIdentity
	if g.ClientID == "" {
		return out, apperror.ErrUnavailable
	}
	if len(credential) > 16384 || len(credential) < 30 {
		return out, apperror.ErrUnauthorized
	}
	p, err := idtoken.Validate(ctx, credential, g.ClientID)
	if err != nil {
		return out, apperror.ErrUnauthorized
	}
	if p.Issuer != "accounts.google.com" && p.Issuer != "https://accounts.google.com" {
		return out, apperror.ErrUnauthorized
	}
	email, _ := p.Claims["email"].(string)
	verified, _ := p.Claims["email_verified"].(bool)
	name, _ := p.Claims["name"].(string)
	hd, _ := p.Claims["hd"].(string)
	if !verified || p.Subject == "" || email == "" || len(email) > 100 {
		return out, apperror.ErrUnauthorized
	}
	if name == "" {
		name = "Google user"
	}
	if len(name) > 100 {
		for len(name) > 100 {
			rr := []rune(name)
			name = string(rr[:len(rr)-1])
		}
	}
	email = strings.ToLower(strings.TrimSpace(email))
	return model.GoogleIdentity{Subject: p.Subject, Email: email, Name: name, Authoritative: strings.HasSuffix(email, "@gmail.com") || hd != ""}, nil
}
