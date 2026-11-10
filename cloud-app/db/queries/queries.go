// Package queries embeds the single SQL source shared by repositories and documentation.
package queries

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.sql
var files embed.FS
var statements = load()

func load() map[string]string {
	out := map[string]string{}
	entries, err := files.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		b, err := files.ReadFile(entry.Name())
		if err != nil {
			panic(err)
		}
		name := ""
		var body strings.Builder
		flush := func() {
			if name != "" {
				if _, exists := out[name]; exists {
					panic("duplicate SQL statement: " + name)
				}
				out[name] = strings.TrimSpace(body.String())
				body.Reset()
			}
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "-- name: ") {
				flush()
				name = strings.TrimSpace(strings.TrimPrefix(line, "-- name: "))
				continue
			}
			if name != "" {
				body.WriteString(line)
				body.WriteByte('\n')
			}
		}
		flush()
	}
	return out
}
func Get(name string) string {
	q, ok := statements[name]
	if !ok {
		panic(fmt.Sprintf("missing SQL statement: %s", name))
	}
	return q
}
func Names() []string {
	out := []string{}
	for n := range statements {
		out = append(out, n)
	}
	return out
}
