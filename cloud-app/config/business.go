package config

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

func loadBusiness(c *Config) error {
	c.FrontendURL = strings.TrimRight(env("FRONTEND_PUBLIC_URL", "https://dev.megaallstararena.com"), "/")
	u, e := url.Parse(c.FrontendURL)
	if e != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("FRONTEND_PUBLIC_URL must be an explicit origin")
	}
	local := u.Hostname() == "localhost"
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		local = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return fmt.Errorf("FRONTEND_PUBLIC_URL must use HTTPS except loopback development")
	}
	c.CookieSecure = u.Scheme == "https"
	c.CookieName = "kicau_refresh"
	if c.CookieSecure {
		c.CookieName = "__Host-kicau_refresh"
	}
	c.GoogleClientID = strings.TrimSpace(env("GOOGLE_CLIENT_ID", ""))
	for _, raw := range strings.Split(env("TRUSTED_PROXIES", "127.0.0.1/32,::1/128"), ",") {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, _, e := net.ParseCIDR(v); e != nil {
			return fmt.Errorf("TRUSTED_PROXIES must contain CIDRs")
		}
		c.TrustedProxies = append(c.TrustedProxies, v)
	}
	c.SMTPHost = env("SMTP_HOST", "")
	c.SMTPPort = env("SMTP_PORT", "587")
	c.SMTPUser = env("SMTP_USER", "")
	c.SMTPPassword = env("SMTP_PASSWORD", "")
	c.SMTPFrom = env("SMTP_FROM", "")
	c.SMTPMode = env("SMTP_MODE", "starttls")
	if c.SMTPMode != "starttls" && c.SMTPMode != "tls" {
		return fmt.Errorf("SMTP_MODE must be starttls or tls")
	}
	n, e := strconv.Atoi(c.SMTPPort)
	if e != nil || n < 1 || n > 65535 {
		return fmt.Errorf("Invalid SMTP_PORT")
	}
	if c.SMTPFrom != "" {
		a, e := mail.ParseAddress(c.SMTPFrom)
		if e != nil || a.Address != c.SMTPFrom {
			return fmt.Errorf("SMTP_FROM must be an email address")
		}
	}
	c.SMTPAllowInsecure, e = strconv.ParseBool(env("SMTP_ALLOW_INSECURE", "false"))
	if e != nil {
		return fmt.Errorf("Invalid SMTP_ALLOW_INSECURE")
	}
	if c.SMTPAllowInsecure {
		ip := net.ParseIP(c.SMTPHost)
		development := env("APP_ENV", "development")
		if (c.SMTPHost != "localhost" && (ip == nil || !ip.IsLoopback())) || (development != "development" && development != "local") {
			return fmt.Errorf("Insecure SMTP is allowed only for loopback development")
		}
	}
	c.MVRefreshInterval, e = time.ParseDuration(env("MV_REFRESH_INTERVAL", "60s"))
	if e != nil || c.MVRefreshInterval < time.Second || c.MVRefreshInterval > time.Hour {
		return fmt.Errorf("MV_REFRESH_INTERVAL must be 1s..1h")
	}
	c.MVRefreshTimeout, e = time.ParseDuration(env("MV_REFRESH_TIMEOUT", "2m"))
	if e != nil || c.MVRefreshTimeout < time.Second || c.MVRefreshTimeout > 10*time.Minute {
		return fmt.Errorf("MV_REFRESH_TIMEOUT must be 1s..10m")
	}
	return nil
}
