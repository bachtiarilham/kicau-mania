// Package generated embeds the contract generated from mounted schema-010 routes.
package generated

import (
	"embed"
	"github.com/swaggo/swag"
)

//go:embed swagger.json
var files embed.FS

func init() {
	b, e := files.ReadFile("swagger.json")
	if e != nil {
		panic(e)
	}
	spec := &swag.Spec{InfoInstanceName: "swagger", SwaggerTemplate: string(b), LeftDelim: "{{", RightDelim: "}}"}
	swag.Register(spec.InstanceName(), spec)
}
