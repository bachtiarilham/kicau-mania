// Package config loads environment configuration without initializing services.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/joho/godotenv"
)

type Config struct {
	MediaS3Endpoint, MediaS3Region, MediaS3Bucket, MediaS3Key, MediaS3Secret string
	WhatsAppEndpoint, WhatsAppAPIKey                                         string
	HTTPAddress                                                              string
	DatabaseURL                                                              string
	RedisURL                                                                 string
	DBMaxConns, DBMinConns                                                   int32
	DBConnectTimeout, DBHealthTimeout                                        time.Duration
	RequestTimeout, ShutdownTimeout                                          time.Duration
	AllowedOrigins                                                           []string
	TrustedProxies                                                           []string
	FrontendURL, CookieName                                                  string
	CookieSecure                                                             bool
}

func Load(path string) (Config, error) {
	if err := godotenv.Load(path); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("cannot load environment file")
	}
	c := Config{HTTPAddress: env("HTTP_ADDR", "127.0.0.1:8091"), DatabaseURL: strings.TrimSpace(env("DATABASE_URL", ""))}
	c.MediaS3Endpoint = env("MEDIA_S3_ENDPOINT", "")
	c.MediaS3Region = env("MEDIA_S3_REGION", "us-east-1")
	c.MediaS3Bucket = env("MEDIA_S3_BUCKET", "")
	c.MediaS3Key = env("MEDIA_S3_ACCESS_KEY", "")
	c.MediaS3Secret = env("MEDIA_S3_SECRET_KEY", "")
	c.WhatsAppEndpoint = env("WHATSAPP_ENDPOINT", "")
	c.WhatsAppAPIKey = env("WHATSAPP_API_KEY", "")
	if c.MediaS3Bucket != "" && (c.MediaS3Key == "" || c.MediaS3Secret == "") {
		return c, fmt.Errorf("MEDIA_S3_ACCESS_KEY and MEDIA_S3_SECRET_KEY are required with MEDIA_S3_BUCKET")
	}

	c.RedisURL = strings.TrimSpace(env("REDIS_URL", ""))
	_, port, err := net.SplitHostPort(c.HTTPAddress)
	if err != nil {
		return c, fmt.Errorf("HTTP_ADDR must contain host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("HTTP_ADDR port must be 1..65535")
	}
	max, err := strconv.ParseInt(env("DB_MAX_CONNS", "10"), 10, 32)
	if err != nil || max < 1 || max > 1000 {
		return c, fmt.Errorf("DB_MAX_CONNS must be 1..1000")
	}
	min, err := strconv.ParseInt(env("DB_MIN_CONNS", "0"), 10, 32)
	if err != nil || min < 0 || min > max {
		return c, fmt.Errorf("DB_MIN_CONNS must be 0..DB_MAX_CONNS")
	}
	c.DBMaxConns, c.DBMinConns = int32(max), int32(min)
	for _, field := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"DB_CONNECT_TIMEOUT", "5s", &c.DBConnectTimeout},
		{"DB_HEALTH_TIMEOUT", "2s", &c.DBHealthTimeout},
		{"HTTP_REQUEST_TIMEOUT", "15s", &c.RequestTimeout},
		{"SHUTDOWN_TIMEOUT", "10s", &c.ShutdownTimeout},
	} {
		value, err := time.ParseDuration(env(field.key, field.fallback))
		if err != nil || value < time.Second || value > time.Minute {
			return c, fmt.Errorf("%s must be 1s..1m", field.key)
		}
		*field.target = value
	}
	seen := map[string]bool{}
	for _, raw := range strings.Split(env("ALLOWED_ORIGINS", "https://dev.megaallstararena.com"), ",") {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return c, fmt.Errorf("ALLOWED_ORIGINS must contain explicit HTTP(S) origins without paths")
		}
		if !seen[origin] {
			c.AllowedOrigins = append(c.AllowedOrigins, origin)
			seen[origin] = true
		}
	}
	if err := loadWebSecurity(&c); err != nil {
		return c, err
	}
	return c, nil
}

func loadWebSecurity(c *Config) error {
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

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
