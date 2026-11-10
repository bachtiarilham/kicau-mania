package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"kicau.cloud/control/internal/apperror"
)

type MailOptions struct {
	Host, Port, User, Password, From, FrontendURL, Mode string
	AllowInsecure                                       bool
}
type Mailer struct{ Options MailOptions }

func (m Mailer) Ready() bool {
	return m.Options.Host != "" && m.Options.From != "" && m.Options.FrontendURL != ""
}
func (m Mailer) Send(ctx context.Context, to, purpose, token string) error {
	if !m.Ready() {
		return apperror.ErrUnavailable
	}
	o := m.Options
	if strings.ContainsAny(to+o.From, "\r\n") {
		return apperror.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	address := net.JoinHostPort(o.Host, o.Port)
	var conn net.Conn
	var err error
	dial := net.Dialer{}
	cfg := &tls.Config{ServerName: o.Host, MinVersion: tls.VersionTLS12}
	if o.Mode == "tls" {
		td := tls.Dialer{NetDialer: &dial, Config: cfg}
		conn, err = td.DialContext(ctx, "tcp", address)
	} else {
		conn, err = dial.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return apperror.ErrUnavailable
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, o.Host)
	if err != nil {
		return apperror.ErrUnavailable
	}
	defer client.Close()
	if o.Mode != "tls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(cfg); err != nil {
				return apperror.ErrUnavailable
			}
		} else if !o.AllowInsecure {
			return apperror.ErrUnavailable
		}
	}
	if o.User != "" {
		if err = client.Auth(smtp.PlainAuth("", o.User, o.Password, o.Host)); err != nil {
			return apperror.ErrUnavailable
		}
	}
	if err = client.Mail(o.From); err != nil {
		return apperror.ErrUnavailable
	}
	if err = client.Rcpt(to); err != nil {
		return apperror.ErrUnavailable
	}
	w, err := client.Data()
	if err != nil {
		return apperror.ErrUnavailable
	}
	path := "/verify-email"
	subject := "Verify your email"
	if purpose == "RESET_PASSWORD" {
		path = "/reset-password"
		subject = "Reset your password"
	}
	link := strings.TrimRight(o.FrontendURL, "/") + path + "#token=" + url.QueryEscape(token)
	_, err = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", o.From, to, subject, link)
	if err != nil {
		return apperror.ErrUnavailable
	}
	if err = w.Close(); err != nil {
		return apperror.ErrUnavailable
	}
	return client.Quit()
}
