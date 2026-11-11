// Package docs embeds the REST contract and pinned Swagger UI assets in the binary.
package docs

import "embed"

// Files contains public documentation only; credentials and environment files are excluded.
//
//go:embed openapi.json swagger/*
var Files embed.FS
