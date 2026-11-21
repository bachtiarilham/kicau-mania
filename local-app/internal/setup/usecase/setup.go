// Package usecase coordinates setup through settings, discovery, probe and runtime interfaces.
package usecase

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/setup/model"
)

type Settings interface {
	Load(context.Context) (model.Configuration, error)
	Save(context.Context, model.Configuration) error
}
type Discovery interface {
	Networks() []model.Network
	Discover(context.Context, string) ([]model.Device, error)
	Profiles(context.Context, model.DiscoveryRequest) ([]model.Profile, error)
}
type Probe interface {
	Probe(context.Context, string) (model.Probe, error)
	Check(context.Context) []model.Diagnostic
}
type Runtime interface {
	CanConfigure() error
	Apply(context.Context, model.Configuration) error
	Status(context.Context) model.RuntimeStatus
}
type Jobs interface {
	Report(context.Context, string)
	Submit(string, func(context.Context) (any, error)) (model.Job, error)
	Get(string) (model.Job, error)
	Cancel(string) error
	Remember(string) string
	Resolve(string) (string, error)
}
type UseCase struct {
	settings  Settings
	discovery Discovery
	probe     Probe
	runtime   Runtime
	jobs      Jobs
}

func New(s Settings, d Discovery, p Probe, r Runtime, j Jobs) *UseCase {
	return &UseCase{s, d, p, r, j}
}
func (u *UseCase) Status(ctx context.Context) model.RuntimeStatus { return u.runtime.Status(ctx) }
func (u *UseCase) Networks() []model.Network                      { return u.discovery.Networks() }
func (u *UseCase) Guidance(code string) model.Diagnostic {
	return model.Diagnostic{Code: code, Advice: model.Advice(code)}
}
func (u *UseCase) Check(ctx context.Context) []model.Diagnostic {
	v := u.probe.Check(ctx)
	for i := range v {
		if v[i].Code != "OK" {
			v[i].Advice = model.Advice(v[i].Code)
		}
	}
	return v
}
func (u *UseCase) Configuration(ctx context.Context) (model.Configuration, error) {
	c, e := u.settings.Load(ctx)
	return model.Public(c), e
}
func (u *UseCase) Job(id string) (model.Job, error) { return u.jobs.Get(id) }
func (u *UseCase) Cancel(id string) error           { return u.jobs.Cancel(id) }
func (u *UseCase) Start(req model.JobRequest) (model.Job, error) {
	switch req.Action {
	case "recheck":
		return u.jobs.Submit("Memeriksa ulang sumber tersimpan", func(ctx context.Context) (any, error) {
			c, e := u.settings.Load(ctx)
			if e != nil {
				return nil, e
			}
			out := []model.SourceCheck{}
			for _, ch := range c.Channels {
				u.jobs.Report(ctx, "Memeriksa channel "+ch.ID)
				if e := ctx.Err(); e != nil {
					return nil, e
				}
				p, e := u.probe.Probe(ctx, ch.Source)
				check := model.SourceCheck{ID: ch.ID}
				if e == nil {
					check.Probe = &p
				} else {
					check.Code = "SOURCE_UNREACHABLE"
					check.Message = "Stream tidak tersedia"
					var f *model.Failure
					if errors.As(e, &f) {
						check.Code = f.Code
						check.Message = f.Message
					}
					if errors.Is(e, context.DeadlineExceeded) {
						check.Code = "TIMEOUT"
					}
					check.Advice = model.Advice(check.Code)
				}
				out = append(out, check)
			}
			return out, nil
		})
	case "discover":
		return u.jobs.Submit("Mencari perangkat ONVIF", func(ctx context.Context) (any, error) { return u.discovery.Discover(ctx, req.IP) })
	case "profiles":
		return u.jobs.Submit("Membaca profil NVR/kamera", func(ctx context.Context) (any, error) {
			profiles, e := u.discovery.Profiles(ctx, req.DiscoveryRequest)
			if e != nil {
				return nil, e
			}
			for i := range profiles {
				p := &profiles[i]
				p.CredentialRef = u.jobs.Remember(p.Source)
				v, e := url.Parse(p.Source)
				if e != nil {
					return nil, apperror.ErrInvalid
				}
				v.User = nil
				v.RawQuery = ""
				v.Fragment = ""
				p.Source = v.String()
			}
			return profiles, nil
		})
	case "probe":
		return u.jobs.Submit("Memeriksa paket video dan audio", func(ctx context.Context) (any, error) {
			v, e := u.source(ctx, req.Channel)
			if e != nil {
				return nil, e
			}
			return u.probe.Probe(ctx, v.Source)
		})
	case "apply":
		return u.jobs.Submit("Memvalidasi dan menerapkan konfigurasi", func(ctx context.Context) (any, error) { return u.apply(ctx, req.Configuration) })
	default:
		return model.Job{}, apperror.ErrInvalid
	}
}
func (u *UseCase) source(ctx context.Context, c model.Channel) (model.Channel, error) {
	if c.CredentialRef != "" {
		v, e := u.jobs.Resolve(c.CredentialRef)
		if e != nil {
			return c, e
		}
		c.Source = v
	}
	v, e := url.Parse(c.Source)
	if e != nil || v.Hostname() == "" || (v.Scheme != "rtsp" && v.Scheme != "rtsps") || strings.ContainsAny(c.Source, "\r\n") {
		return c, apperror.ErrInvalid
	}
	old, _ := u.settings.Load(ctx)
	for _, o := range old.Channels {
		p, err := url.Parse(o.Source)
		if err == nil && o.ID == c.ID && p.Host == v.Host && p.Path == v.Path {
			if v.RawQuery == "" {
				v.RawQuery = p.RawQuery
			}
			break
		}
	}
	if c.Password != "" || c.Username != "" {
		v.User = url.UserPassword(c.Username, c.Password)
	} else if v.User == nil {
		{
			for _, o := range old.Channels {
				p, e := url.Parse(o.Source)
				if e == nil && o.ID == c.ID && p.Host == v.Host && p.Path == v.Path {
					v.User = p.User
					break
				}
			}
		}
	}
	c.Source = v.String()
	c.Username, c.Password, c.CredentialRef = "", "", ""
	return c, nil
}
func (u *UseCase) apply(ctx context.Context, c model.Configuration) (any, error) {
	if e := u.runtime.CanConfigure(); e != nil {
		return nil, e
	}
	checks := u.probe.Check(ctx)
	required := map[string]bool{"FFmpeg": true, "ffprobe": true, "MediaMTX": true}
	for _, d := range checks {
		if required[d.Component] && d.Code != "OK" {
			return nil, &model.Failure{Code: d.Code, Message: d.Message}
		}
	}
	if len(c.Channels) < 1 || len(c.Channels) > 24 {
		return nil, apperror.ErrInvalid
	}
	ip := net.ParseIP(c.LANIP)
	_, sub, e := net.ParseCIDR(c.LANCIDR)
	if e != nil || ip == nil || ip.To4() == nil || !sub.Contains(ip) {
		return nil, apperror.ErrInvalid
	}
	known := false
	for _, n := range u.discovery.Networks() {
		if n.IP == c.LANIP && n.CIDR == c.LANCIDR {
			known = true
		}
	}
	if !known {
		return nil, apperror.Public(apperror.ErrInvalid, "Pilih interface LAN komputer engine yang masih aktif")
	}
	c.Version = 1
	c.Channels = append([]model.Channel(nil), c.Channels...)
	seen := map[string]bool{}
	for i, v := range c.Channels {
		u.jobs.Report(ctx, "Memeriksa channel "+v.ID)
		if !identifier.Valid(v.ID) || seen[v.ID] {
			return nil, apperror.ErrInvalid
		}
		seen[v.ID] = true
		v, e = u.source(ctx, v)
		if e != nil {
			return nil, e
		}
		p, e := u.probe.Probe(ctx, v.Source)
		if e == nil {
			if e := model.ValidateMedia(p); e != nil {
				return nil, e
			}
			v.Probe = &p
		} else {
			v.Probe = nil
		}
		c.Channels[i] = v
	}
	for _, d := range checks {
		if required[d.Component] && d.Code != "OK" {
			return nil, &model.Failure{Code: d.Code, Message: d.Message}
		}
	}
	u.jobs.Report(ctx, "Menerapkan runtime dan menyimpan konfigurasi")
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	// Runtime owns serialized apply/rollback and durable settings commit.
	if e := u.runtime.Apply(ctx, c); e != nil {
		return nil, e
	}
	return model.Public(c), nil
}
