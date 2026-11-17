// lan-config renders LAN deployment settings without contacting the cloud/router.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	source := flag.String("source", ".env.portable.example", "source application configuration")
	settings := flag.String("settings", ".env.lan", "canonical LAN configuration")
	proxy := flag.Bool("serve-proxy", false, "run the provisioned Caddy binary with DNS-01")
	paths := flag.Bool("paths", false, "render configuration and print only generated paths as JSON")
	flag.Parse()
	values, err := godotenv.Read(*settings)
	if os.IsNotExist(err) {
		values, err = godotenv.Read(*source)
		if err != nil {
			return fmt.Errorf("source env unavailable")
		}
		defaults, err := godotenv.Read(".env.lan.example")
		if err != nil {
			return err
		}
		for key, value := range defaults {
			values[key] = value
		}
		if values["API_TOKEN"] == "" || strings.HasPrefix(values["API_TOKEN"], "REPLACE_") {
			return fmt.Errorf("initialize a valid source API_TOKEN first")
		}
		text, err := godotenv.Marshal(values)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(*settings, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.WriteString(text + "\n")
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	} else if err != nil {
		return fmt.Errorf("cannot read LAN env")
	}
	for key := range values {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}
	public, err := url.Parse(values["LOCAL_PUBLIC_URL"])
	if err != nil || public.Scheme != "https" || public.Hostname() == "" || public.User != nil || public.Path != "" || public.RawQuery != "" || public.Fragment != "" {
		return fmt.Errorf("LOCAL_PUBLIC_URL must be an HTTPS origin")
	}
	for _, r := range public.Hostname() {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-') {
			return fmt.Errorf("invalid local hostname")
		}
	}
	ip := net.ParseIP(values["LAN_SERVER_IP"])
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("LAN_SERVER_IP must be IPv4")
	}
	if _, subnet, err := net.ParseCIDR(values["LAN_READ_CIDR"]); err != nil || subnet.IP.To4() == nil || !subnet.Contains(ip) {
		return fmt.Errorf("LAN_READ_CIDR must be an IPv4 subnet containing LAN_SERVER_IP")
	}
	frontend, err := url.Parse(values["FRONTEND_ORIGIN"])
	if err != nil || frontend.Scheme != "https" || frontend.Host == "" || frontend.User != nil || frontend.Path != "" || frontend.RawQuery != "" || frontend.Fragment != "" {
		return fmt.Errorf("FRONTEND_ORIGIN must be an HTTPS origin")
	}
	if values["DNS_PROVIDER"] != "hostinger" {
		return fmt.Errorf("automatic DNS-01 currently requires the Hostinger module; configure another provider explicitly before deployment")
	}
	runtimeDir, err := filepath.Abs(values["RUNTIME_DIR"])
	if err != nil {
		return err
	}
	dataDir, err := filepath.Abs(values["DATA_DIR"])
	if err != nil {
		return err
	}
	if runtimeDir == "" || values["RUNTIME_DIR"] == "" || values["DATA_DIR"] == "" {
		return fmt.Errorf("DATA_DIR and RUNTIME_DIR are required")
	}
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return err
	}
	app := map[string]string{}
	for key, value := range values {
		app[key] = value
	}
	// Provider credentials belong to the proxy process, never the application env.
	delete(app, "HOSTINGER_API_TOKEN")
	app["HTTP_ADDR"] = "127.0.0.1:8090"
	app["RTSP_ADDR"] = "127.0.0.1:8554"
	app["RTSP_BASE_URL"] = "rtsp://127.0.0.1:8554"
	app["MEDIA_READ_CIDRS"] = "127.0.0.1/32,::1/128," + values["LAN_READ_CIDR"]
	app["DNS_LISTEN_ADDR"] = net.JoinHostPort(ip.String(), "53")
	app["ALLOWED_ORIGINS"] = values["FRONTEND_ORIGIN"] + "," + values["LOCAL_PUBLIC_URL"]
	if !*proxy {
		if err := godotenv.Write(app, filepath.Join(runtimeDir, "app.env")); err != nil {
			return err
		}
		if err := os.Chmod(filepath.Join(runtimeDir, "app.env"), 0600); err != nil {
			return err
		}
	}
	certDir := filepath.ToSlash(filepath.Join(dataDir, "caddy"))
	caddy := fmt.Sprintf(`{
    admin off
    auto_https disable_redirects
    email {env.ACME_EMAIL}
    storage file_system {
        root %q
    }
}
%s {
    bind %s
    tls {
        dns hostinger {env.HOSTINGER_API_TOKEN}
    }
    @outside not remote_ip 127.0.0.1 ::1 %s
    respond @outside 403
    @application path / /api/v1/* /healthz /readyz
    handle @application {
        reverse_proxy 127.0.0.1:8090
    }
    handle {
        respond 404
    }
}
`, certDir, values["LOCAL_PUBLIC_URL"], ip.String(), values["LAN_READ_CIDR"])
	configPath := filepath.Join(runtimeDir, "Caddyfile")
	if !*proxy {
		if err := os.WriteFile(configPath, []byte(caddy), 0600); err != nil {
			return err
		}
	}
	if *paths {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"app_env": filepath.Join(runtimeDir, "app.env"), "caddyfile": configPath})
	}
	if !*proxy {
		fmt.Println("LAN configuration rendered. Router/DNS records were not changed.")
		fmt.Printf("Hostinger A record: %s -> %s\n", public.Hostname(), ip.String())
		fmt.Printf("%s %s: reserve server IP and set DHCP DNS to %s (no public secondary DNS).\n", values["ROUTER_VENDOR"], values["ROUTER_MODEL"], ip.String())
		fmt.Println("Start the built application with -env", filepath.Join(runtimeDir, "app.env"))
		return nil
	}
	if values["HOSTINGER_API_TOKEN"] == "" || values["ACME_EMAIL"] == "" {
		return fmt.Errorf("set HOSTINGER_API_TOKEN and ACME_EMAIL in the private LAN env before certificate provisioning")
	}
	binary := values["CADDY_BIN"]
	if binary == "" {
		return fmt.Errorf("CADDY_BIN is required")
	}
	modules, err := exec.Command(binary, "list-modules").Output()
	if err != nil || !strings.Contains(string(modules), "dns.providers.hostinger") {
		return fmt.Errorf("Caddy binary must include dns.providers.hostinger")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "--config", configPath, "--adapter", "caddyfile")
	command.Env = append(os.Environ(), "HOSTINGER_API_TOKEN="+values["HOSTINGER_API_TOKEN"], "ACME_EMAIL="+values["ACME_EMAIL"])
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
