package network

import "net"

type Network struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	CIDR string `json:"cidr"`
}

func Interfaces() []Network {
	out := []Network{}
	ifs, _ := net.Interfaces()
	for _, v := range ifs {
		if v.Flags&net.FlagUp == 0 || v.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := v.Addrs()
		for _, a := range addrs {
			ip, sub, e := net.ParseCIDR(a.String())
			if e == nil && ip.To4() != nil && !ip.IsLinkLocalUnicast() {
				out = append(out, Network{Name: v.Name, IP: ip.String(), CIDR: sub.String()})
			}
		}
	}
	return out
}
