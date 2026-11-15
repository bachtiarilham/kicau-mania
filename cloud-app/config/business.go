package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
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
	return nil
}
