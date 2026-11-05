// Package dns resolves the arena hostname without an internet dependency.
// Other names are forwarded to configured upstream resolvers when available.
package dns

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	mdns "github.com/miekg/dns"
)

type Options struct {
	Address, Host, IP       string
	Upstreams, AllowedCIDRs []string
}
type Server struct{ udp, tcp *mdns.Server }

func Start(opts Options) (*Server, error) {
	ip := net.ParseIP(opts.IP)
	if ip == nil || ip.To4() == nil {
		return nil, fmt.Errorf("DNS requires a local IPv4 address")
	}
	allowed := []netip.Prefix{}
	for _, raw := range opts.AllowedCIDRs {
		p, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		allowed = append(allowed, p)
	}
	host := mdns.Fqdn(strings.ToLower(opts.Host))
	udp, err := net.ListenPacket("udp", opts.Address)
	if err != nil {
		return nil, err
	}
	tcp, err := net.Listen("tcp", opts.Address)
	if err != nil {
		udp.Close()
		return nil, err
	}
	slots := make(chan struct{}, 128)
	handler := mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
		response := new(mdns.Msg)
		response.SetReply(r)
		response.RecursionAvailable = true
		remote, _, err := net.SplitHostPort(w.RemoteAddr().String())
		source, parseErr := netip.ParseAddr(remote)
		permit := false
		if err == nil && parseErr == nil {
			for _, p := range allowed {
				if p.Contains(source.Unmap()) {
					permit = true
					break
				}
			}
		}
		if !permit || len(r.Question) != 1 {
			response.Rcode = mdns.RcodeRefused
			_ = w.WriteMsg(response)
			return
		}
		q := r.Question[0]
		if strings.EqualFold(q.Name, host) {
			response.Authoritative = true
			if q.Qclass == mdns.ClassINET && (q.Qtype == mdns.TypeA || q.Qtype == mdns.TypeANY) {
				response.Answer = []mdns.RR{&mdns.A{Hdr: mdns.RR_Header{Name: q.Name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 60}, A: ip.To4()}}
			}
			_ = w.WriteMsg(response)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			response.Rcode = mdns.RcodeServerFailure
			_ = w.WriteMsg(response)
			return
		}
		client := &mdns.Client{Net: "udp", Timeout: 2 * time.Second}
		for _, upstream := range opts.Upstreams {
			answer, _, err := client.Exchange(r, upstream)
			if err == nil && answer.Truncated {
				tcpClient := &mdns.Client{Net: "tcp", Timeout: 2 * time.Second}
				answer, _, err = tcpClient.Exchange(r, upstream)
			}
			if err == nil {
				_ = w.WriteMsg(answer)
				return
			}
		}
		response.Rcode = mdns.RcodeServerFailure
		_ = w.WriteMsg(response)
	})
	s := &Server{udp: &mdns.Server{PacketConn: udp, Handler: handler}, tcp: &mdns.Server{Listener: tcp, Handler: handler}}
	go s.udp.ActivateAndServe()
	go s.tcp.ActivateAndServe()
	return s, nil
}
func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.udp.ShutdownContext(ctx)
	_ = s.tcp.ShutdownContext(ctx)
}
