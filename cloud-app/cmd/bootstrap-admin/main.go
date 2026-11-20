// Bootstrap creates an explicit verified superadmin; it never prints credentials.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"kicau.cloud/control/config"
	"kicau.cloud/control/internal/app"
)

func main() {
	path := flag.String("env", ".env", "environment configuration file")
	flag.Parse()
	cfg, e := config.Load(*path)
	if e != nil {
		fmt.Fprintln(os.Stderr, "Invalid configuration")
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, e := app.New(ctx, cfg)
	if e != nil {
		fmt.Fprintln(os.Stderr, "Database setup unavailable")
		os.Exit(1)
	}
	defer c.Close()
	e = c.User.Bootstrap(ctx, os.Getenv("BOOTSTRAP_ADMIN_NAME"), os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PHONE"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
	if e != nil {
		fmt.Fprintln(os.Stderr, "Bootstrap rejected; check schema, account uniqueness, and input")
		os.Exit(1)
	}
	fmt.Println("ADMIN created")
}
