// Package onvif implements bounded, read-only ONVIF discovery and Media1/Media2 queries.
package onvif

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"kicau.local/edge/internal/setup/model"
)

type Client struct{}

func (Client) Networks() []model.Network {
	out := []model.Network{}
	ifs, _ := net.Interfaces()
	for _, v := range ifs {
		if v.Flags&net.FlagUp == 0 || v.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := v.Addrs()
		for _, a := range addrs {
			ip, sub, e := net.ParseCIDR(a.String())
			if e == nil && ip.To4() != nil && !ip.IsLinkLocalUnicast() {
				out = append(out, model.Network{Name: v.Name, IP: ip.String(), CIDR: sub.String()})
			}
		}
	}
	return out
}
func (c Client) subnet(selected string) (net.IP, *net.IPNet, error) {
	for _, n := range c.Networks() {
		if n.IP == selected {
			_, sub, _ := net.ParseCIDR(n.CIDR)
			return net.ParseIP(n.IP), sub, nil
		}
	}
	return nil, nil, &model.Failure{Code: "NETWORK_UNAVAILABLE", Message: "Pilih interface LAN komputer engine yang aktif"}
}
func (c Client) endpoint(raw, selected string) error {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return &model.Failure{Code: "INVALID_ENDPOINT", Message: "Alamat layanan ONVIF harus HTTP/HTTPS tanpa kredensial pada URL"}
	}
	_, sub, e := c.subnet(selected)
	if e != nil {
		return e
	}
	ips, e := net.LookupIP(u.Hostname())
	if e != nil || len(ips) == 0 {
		return &model.Failure{Code: "DEVICE_UNREACHABLE", Message: "Alamat perangkat tidak dapat ditemukan"}
	}
	for _, ip := range ips {
		if !sub.Contains(ip) {
			return &model.Failure{Code: "NETWORK_SCOPE", Message: "Perangkat harus berada pada subnet LAN yang dipilih"}
		}
	}
	return nil
}
func (c Client) Discover(ctx context.Context, selected string) ([]model.Device, error) {
	ip, _, e := c.subnet(selected)
	if e != nil {
		return nil, e
	}
	conn, e := net.ListenUDP("udp4", &net.UDPAddr{IP: ip})
	if e != nil {
		return nil, &model.Failure{Code: "DISCOVERY_BLOCKED", Message: "Tidak dapat membuka WS-Discovery. Periksa interface dan firewall UDP 3702"}
	}
	defer conn.Close()
	var nonce [16]byte
	rand.Read(nonce[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", nonce[:4], nonce[4:6], nonce[6:8], nonce[8:10], nonce[10:])
	msg := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery"><s:Header><a:MessageID>uuid:` + id + `</a:MessageID><a:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</a:To><a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</a:Action></s:Header><s:Body><d:Probe/></s:Body></s:Envelope>`
	if _, e = conn.WriteToUDP([]byte(msg), &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 3702}); e != nil {
		return nil, &model.Failure{Code: "DISCOVERY_BLOCKED", Message: "Multicast discovery gagal; masukkan alamat ONVIF atau RTSP manual"}
	}
	deadline := time.Now().Add(4 * time.Second)
	out := []model.Device{}
	seen := map[string]bool{}
	buf := make([]byte, 65535)
	for time.Now().Before(deadline) && len(out) < 32 {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, _, e := conn.ReadFromUDP(buf)
		if e != nil {
			if t, ok := e.(net.Error); ok && t.Timeout() {
				continue
			}
			break
		}
		root, e := parse(buf[:n])
		if e != nil {
			continue
		}
		for _, node := range root.all("XAddrs") {
			for _, address := range strings.Fields(node.Text) {
				if seen[address] || c.endpoint(address, selected) != nil {
					continue
				}
				seen[address] = true
				u, _ := url.Parse(address)
				out = append(out, model.Device{Endpoint: address, Name: u.Hostname()})
			}
		}
	}
	return out, nil
}

type node struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Text     string     `xml:",chardata"`
	Children []node     `xml:",any"`
}

func parse(b []byte) (node, error) { var n node; e := xml.Unmarshal(b, &n); return n, e }
func (n node) all(name string) []node {
	out := []node{}
	if n.XMLName.Local == name {
		out = append(out, n)
	}
	for _, v := range n.Children {
		out = append(out, v.all(name)...)
	}
	return out
}
func (n node) value(name string) string {
	v := n.all(name)
	if len(v) > 0 {
		return strings.TrimSpace(v[0].Text)
	}
	return ""
}
func (n node) attr(name string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
func escape(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
func header(user, password string) string {
	if user == "" {
		return ""
	}
	var nonce [24]byte
	rand.Read(nonce[:])
	created := time.Now().UTC().Format(time.RFC3339Nano)
	sum := sha1.Sum(append(append(append([]byte{}, nonce[:]...), []byte(created)...), []byte(password)...))
	return `<s:Header><wsse:Security xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" xmlns:wsu="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"><wsse:UsernameToken><wsse:Username>` + escape(user) + `</wsse:Username><wsse:Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">` + base64.StdEncoding.EncodeToString(sum[:]) + `</wsse:Password><wsse:Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">` + base64.StdEncoding.EncodeToString(nonce[:]) + `</wsse:Nonce><wsu:Created>` + created + `</wsu:Created></wsse:UsernameToken></wsse:Security></s:Header>`
}

// HTTP Digest covers ONVIF devices using HTTP authentication in addition to WS-Security.
func digest(challenge, user, password, method, uri string) string {
	if !strings.HasPrefix(challenge, "Digest ") {
		return ""
	}
	values := map[string]string{}
	s := strings.TrimPrefix(challenge, "Digest ")
	for len(s) > 0 {
		s = strings.TrimLeft(s, " ,")
		k, rest, ok := strings.Cut(s, "=")
		if !ok {
			break
		}
		var value string
		if strings.HasPrefix(rest, "\"") {
			var end int
			for end = 1; end < len(rest); end++ {
				if rest[end] == '"' && rest[end-1] != '\\' {
					break
				}
			}
			if end >= len(rest) {
				break
			}
			value = rest[1:end]
			s = rest[end+1:]
		} else {
			value, s, _ = strings.Cut(rest, ",")
		}
		values[strings.TrimSpace(k)] = value
	}
	if values["nonce"] == "" || values["realm"] == "" || (values["algorithm"] != "" && strings.ToUpper(values["algorithm"]) != "MD5") {
		return ""
	}
	hash := func(s string) string { b := md5.Sum([]byte(s)); return hex.EncodeToString(b[:]) }
	a1 := hash(user + ":" + values["realm"] + ":" + password)
	a2 := hash(method + ":" + uri)
	var random [16]byte
	rand.Read(random[:])
	cnonce := hex.EncodeToString(random[:])
	response := hash(a1 + ":" + values["nonce"] + ":" + a2)
	qop := ""
	for _, v := range strings.Split(values["qop"], ",") {
		if strings.TrimSpace(v) == "auth" {
			qop = "auth"
		}
	}
	if values["qop"] != "" && qop == "" {
		return ""
	}
	if qop != "" {
		response = hash(a1 + ":" + values["nonce"] + ":00000001:" + cnonce + ":auth:" + a2)
	}
	q := func(s string) string { return strconv.Quote(s) }
	out := `Digest username=` + q(user) + `, realm=` + q(values["realm"]) + `, nonce=` + q(values["nonce"]) + `, uri=` + q(uri) + `, response=` + q(response)
	if qop != "" {
		out += `, qop=auth, nc=00000001, cnonce=` + q(cnonce)
	}
	if values["opaque"] != "" {
		out += `, opaque=` + q(values["opaque"])
	}
	return out
}
func (c Client) soap(ctx context.Context, request model.DiscoveryRequest, endpoint, action, body string) (node, error) {
	if e := c.endpoint(endpoint, request.IP); e != nil {
		return node{}, e
	}
	client := &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	payload := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:tt="http://www.onvif.org/ver10/schema">` + header(request.Username, request.Password) + `<s:Body>` + body + `</s:Body></s:Envelope>`
	auth := ""
	for attempt := 0; attempt < 2; attempt++ {
		r, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(payload))
		if e != nil {
			return node{}, &model.Failure{Code: "INVALID_ENDPOINT", Message: "Alamat ONVIF tidak valid"}
		}
		r.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+action+`"`)
		r.Header.Set("SOAPAction", `"`+action+`"`)
		if auth != "" {
			r.Header.Set("Authorization", auth)
		} else if request.Username != "" {
			r.SetBasicAuth(request.Username, request.Password)
		}
		res, e := client.Do(r)
		if e != nil {
			if ctx.Err() != nil {
				return node{}, ctx.Err()
			}
			return node{}, &model.Failure{Code: "DEVICE_UNREACHABLE", Message: "Layanan ONVIF tidak terjangkau; periksa IP, port, subnet, dan sertifikat"}
		}
		b, e := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
		res.Body.Close()
		if e != nil || len(b) > 2*1024*1024 {
			return node{}, &model.Failure{Code: "DEVICE_PROTOCOL", Message: "Respons ONVIF tidak valid atau terlalu besar"}
		}
		if res.StatusCode == 401 && attempt == 0 {
			auth = digest(res.Header.Get("WWW-Authenticate"), request.Username, request.Password, r.Method, r.URL.RequestURI())
			if auth != "" {
				continue
			}
		}
		if res.StatusCode == 401 || res.StatusCode == 403 {
			return node{}, &model.Failure{Code: "DEVICE_AUTH_FAILED", Message: "Kredensial ONVIF ditolak. Periksa akun ONVIF khusus dan sinkronisasi waktu perangkat"}
		}
		n, e := parse(b)
		if e != nil || len(n.all("Fault")) > 0 || res.StatusCode != 200 {
			return node{}, &model.Failure{Code: "DEVICE_PROTOCOL", Message: "Operasi ONVIF tidak didukung atau ditolak; gunakan profil lain atau RTSP manual"}
		}
		return n, nil
	}
	return node{}, &model.Failure{Code: "DEVICE_AUTH_FAILED", Message: "Autentikasi ONVIF gagal"}
}
func (c Client) Profiles(ctx context.Context, r model.DiscoveryRequest) ([]model.Profile, error) {
	if len(r.Username) > 256 || len(r.Password) > 1024 {
		return nil, &model.Failure{Code: "INVALID_CREDENTIALS", Message: "Kredensial terlalu panjang"}
	}
	services, e := c.soap(ctx, r, r.Endpoint, "http://www.onvif.org/ver10/device/wsdl/GetServices", `<GetServices xmlns="http://www.onvif.org/ver10/device/wsdl"><IncludeCapability>false</IncludeCapability></GetServices>`)
	type mediaService struct{ address, namespace string }
	available := []mediaService{}
	if e == nil {
		for _, v := range services.all("Service") {
			ns := v.value("Namespace")
			if ns == "http://www.onvif.org/ver20/media/wsdl" || ns == "http://www.onvif.org/ver10/media/wsdl" {
				available = append(available, mediaService{v.value("XAddr"), ns})
			}
		}
	}
	if len(available) == 0 {
		caps, err := c.soap(ctx, r, r.Endpoint, "http://www.onvif.org/ver10/device/wsdl/GetCapabilities", `<GetCapabilities xmlns="http://www.onvif.org/ver10/device/wsdl"><Category>Media</Category></GetCapabilities>`)
		if err != nil {
			return nil, err
		}
		for _, v := range caps.all("Media") {
			if address := v.value("XAddr"); address != "" {
				available = append(available, mediaService{address, "http://www.onvif.org/ver10/media/wsdl"})
			}
		}
	}
	out := []model.Profile{}
	sort.SliceStable(available, func(i, j int) bool {
		return strings.Contains(available[i].namespace, "ver20") && !strings.Contains(available[j].namespace, "ver20")
	})
	var last error
	for _, svc := range available {
		profiles, e := c.soap(ctx, r, svc.address, svc.namespace+"/GetProfiles", `<GetProfiles xmlns="`+svc.namespace+`"/>`)
		if e != nil {
			last = e
			continue
		}
		for _, v := range profiles.all("Profiles") {
			if len(out) >= 96 {
				break
			}
			token := v.attr("token")
			if token == "" {
				continue
			}
			body := `<GetStreamUri xmlns="` + svc.namespace + `"><StreamSetup><tt:Stream>RTP-Unicast</tt:Stream><tt:Transport><tt:Protocol>RTSP</tt:Protocol></tt:Transport></StreamSetup><ProfileToken>` + escape(token) + `</ProfileToken></GetStreamUri>`
			if strings.Contains(svc.namespace, "ver20") {
				body = `<GetStreamUri xmlns="` + svc.namespace + `"><Protocol>RTSP</Protocol><ProfileToken>` + escape(token) + `</ProfileToken></GetStreamUri>`
			}
			uri, e := c.soap(ctx, r, svc.address, svc.namespace+"/GetStreamUri", body)
			if e != nil {
				last = e
				continue
			}
			source := uri.value("Uri")
			u, e := url.Parse(source)
			if e != nil || u.Hostname() == "" || (u.Scheme != "rtsp" && u.Scheme != "rtsps") {
				continue
			}
			// Never forward credentials to another host outside the explicitly selected LAN.
			endpoint := *u
			endpoint.User = nil
			endpoint.Scheme = "http"
			if c.endpoint(endpoint.String(), r.IP) != nil {
				continue
			}
			if r.Username != "" && u.User == nil {
				u.User = url.UserPassword(r.Username, r.Password)
			}
			w, _ := strconv.Atoi(v.value("Width"))
			h, _ := strconv.Atoi(v.value("Height"))
			out = append(out, model.Profile{Token: token, Name: v.value("Name"), Source: u.String(), VideoCodec: strings.ToLower(v.value("Encoding")), Width: w, Height: h})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	if last != nil {
		return nil, last
	}
	return nil, &model.Failure{Code: "NO_PROFILES", Message: fmt.Sprintf("Tidak ada profil RTSP dari %d layanan. Aktifkan ONVIF atau isi RTSP manual", len(available))}
}
