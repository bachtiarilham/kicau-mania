package dns

import (
	"net"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

func TestLocalNameResolvesWithUnavailableInternet(t *testing.T) {
	// Choose a free TCP port; Start also binds UDP at that exact port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	s, err := Start(Options{Address: addr, Host: "lan-dev.example.test", IP: "192.168.2.151", AllowedCIDRs: []string{"127.0.0.1/32"}, Upstreams: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, network := range []string{"udp", "tcp"} {
		client := &mdns.Client{Net: network, Timeout: time.Second}
		q := new(mdns.Msg)
		q.SetQuestion("lan-dev.example.test.", mdns.TypeA)
		res, _, err := client.Exchange(q, addr)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Answer) != 1 || res.Answer[0].(*mdns.A).A.String() != "192.168.2.151" {
			t.Fatal("local DNS depended on upstream")
		}
		q.SetQuestion("cloud.example.test.", mdns.TypeA)
		res, _, err = client.Exchange(q, addr)
		if err != nil || res.Rcode != mdns.RcodeServerFailure {
			t.Fatal("unexpected external resolution", err)
		}
	}
}
