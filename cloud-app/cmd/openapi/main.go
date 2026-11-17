// openapi generates documentation from the routes actually mounted by schema 010.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"kicau.cloud/control/config"
	"kicau.cloud/control/internal/app"
	"kicau.cloud/control/internal/delivery"
	"os"
	"regexp"
	"strings"
)

type object = map[string]any

func main() {
	c, e := app.New(context.Background(), config.Config{})
	if e != nil {
		panic(e)
	}
	defer c.Close()
	r := delivery.Router(delivery.Options{}, c.Handlers)
	paths := object{}
	param := regexp.MustCompile(`:(\w+)`)
	for _, route := range r.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		path := param.ReplaceAllString(route.Path, "{$1}")
		operations, ok := paths[path].(object)
		if !ok {
			operations = object{}
			paths[path] = operations
		}
		params := []any{}
		for _, p := range param.FindAllStringSubmatch(route.Path, -1) {
			v := object{"name": p[1], "in": "path", "required": true, "type": "string"}
			if p[1] == "role" {
				v["enum"] = []string{"ADMIN", "OPERATOR", "JURI"}
			} else {
				v["format"] = "uuid"
			}
			params = append(params, v)
		}
		public := route.Path == "/api/v1/auth/login" || route.Path == "/api/v1/auth/refresh" || route.Path == "/api/v1/auth/logout"
		op := object{"summary": route.Method + " " + path, "tags": []string{strings.Split(strings.TrimPrefix(route.Path, "/api/v1/"), "/")[0]}, "produces": []string{"application/json"}, "responses": object{}}
		if strings.HasPrefix(route.Path, "/api/v1/engine/media/") {
			op["security"] = []any{object{"EngineAuth": []string{}}}
		}
		if public {
			op["security"] = []any{}
		} else {
			op["security"] = []any{object{"BearerAuth": []string{}}}
		}
		if strings.HasPrefix(route.Path, "/api/v1/engine/media/") {
			op["security"] = []any{object{"EngineAuth": []string{}}}
		}
		if route.Path == "/api/v1/auth/refresh" || route.Path == "/api/v1/auth/logout" {
			params = append(params, object{"name": "Origin", "in": "header", "required": true, "type": "string", "description": "Exact frontend origin in ALLOWED_ORIGINS; refresh cookie required."})
		}
		if strings.HasSuffix(route.Path, "/proof") {
			if route.Method == "POST" {
				op["consumes"] = []string{"multipart/form-data"}
				params = append(params, object{"in": "formData", "name": "file", "required": true, "type": "file", "description": "JPEG/PNG signature, decoded image, at most 5 MiB."})
			} else {
				op["produces"] = []string{"image/jpeg", "image/png"}
			}
		} else if route.Method == "POST" || route.Method == "PUT" || route.Method == "PATCH" {
			name := bodyName(route.Path)
			op["consumes"] = []string{"application/json"}
			schema := any(object{"type": "object"})
			if name != "" {
				schema = object{"$ref": "#/definitions/" + name}
			}
			params = append(params, object{"in": "body", "name": "request", "required": true, "schema": schema})
		}
		if strings.Contains(route.Path, "/registrations") && !strings.Contains(route.Path, "/arenas/") && route.Method != "PUT" && !(route.Path == "/api/v1/registrations" && route.Method == "POST") {
			params = append(params, object{"in": "query", "name": "arena_id", "type": "string", "format": "uuid", "required": strings.Contains(route.Path, "/:id")})
		}
		if route.Method == "DELETE" && (strings.HasPrefix(route.Path, "/api/v1/users/") || route.Path == "/api/v1/birds/:id") {
			params = append(params, object{"in": "query", "name": "expected_version", "type": "integer", "required": true})
		}
		if strings.Contains(route.Path, "/birds/:id/awards") && route.Method != "GET" {
			params = append(params, object{"in": "query", "name": "arena_id", "type": "string", "format": "uuid", "description": "Required for internal awards."})
		}
		if route.Method == "GET" && !strings.Contains(route.Path, "/:id") && !strings.HasSuffix(route.Path, "/me") && !strings.HasSuffix(route.Path, "/sessions") {
			params = append(params, object{"in": "query", "name": "cursor", "type": "string"}, object{"in": "query", "name": "limit", "type": "integer", "minimum": 1, "maximum": 100})
		}
		if strings.HasSuffix(route.Path, "/history") {
			params = append(params, object{"in": "query", "name": "after_version", "type": "integer", "minimum": 0}, object{"in": "query", "name": "limit", "type": "integer", "minimum": 1, "maximum": 100})
		}
		if route.Method == "GET" && strings.HasSuffix(route.Path, "/rounds") && strings.Contains(route.Path, "/arenas/") {
			params = append(params, object{"in": "query", "name": "event_id", "type": "string", "format": "uuid"})
		}
		if route.Path == "/api/v1/registrations" && route.Method == "GET" {
			for _, filter := range []string{"round_id", "event_id", "bird_id"} {
				params = append(params, object{"in": "query", "name": filter, "type": "string", "format": "uuid"})
			}
		}
		if route.Path == "/api/v1/users" && route.Method == "GET" {
			for _, filter := range []string{"is_eo", "role"} {
				params = append(params, object{"in": "query", "name": filter, "type": "string"})
			}
		}
		if route.Path == "/api/v1/birds" && route.Method == "GET" {
			params = append(params, object{"in": "query", "name": "owner_id", "type": "string", "format": "uuid"})
		}
		if route.Method == "GET" && (route.Path == "/api/v1/users" || route.Path == "/api/v1/birds") {
			params = append(params, object{"in": "query", "name": "q", "type": "string", "maxLength": 200})
		}
		if route.Path == "/api/v1/media/recordings" {
			for _, key := range []string{"arena_id", "round_id", "session_id", "channel_id"} {
				params = append(params, object{"in": "query", "name": key, "type": "string", "required": key == "arena_id"})
			}
		}
		for _, status := range []string{"200", "400", "401", "403", "404", "409", "429", "503"} {
			op["responses"].(object)[status] = object{"description": map[string]string{"200": "Success after commit", "400": "Invalid fields or business values", "401": "Invalid or revoked session", "403": "Role/arena/roster access denied", "404": "Resource not found", "409": "Version, relationship or workflow conflict", "429": "Login attempt limit", "503": "PostgreSQL/schema/Redis unavailable"}[status], "schema": object{"$ref": "#/definitions/Envelope"}}
		}
		op["parameters"] = params
		if route.Method == "GET" && strings.HasSuffix(route.Path, "/proof") {
			op["responses"].(object)["200"] = object{"description": "Authorized JPEG/PNG bytes", "schema": object{"type": "file"}}
		}
		operations[strings.ToLower(route.Method)] = op
	}
	defs := object{}
	fields := map[string]string{"Login": "email password device_id", "Participant": "name phone email is_eo is_active expected_version", "Bird": "name owner_id bird_species_id expected_version", "Arena": "name address timezone is_active", "Species": "name", "Gantangan": "number description baris kolom", "Event": "owner_id title description event_date status", "Category": "code name", "Round": "event_id category_id bird_species_id round_number title", "Criterion": "criteria_name", "Registration": "user_id arena_id round_id bird_id gantangan_id entry_fee payment_method", "Transfer": "owner_id expected_version notes", "Award": "registration_id award_name event_name organizer_name rank awarded_at certificate_url", "Transition": "expected_status", "TicketAction": "reason", "Password": "old_password new_password", "Credentials": "password"}
	for name, keys := range fields {
		p := object{}
		for _, key := range strings.Fields(keys) {
			typ := "string"
			if strings.HasPrefix(key, "is_") {
				typ = "boolean"
			}
			if key == "expected_version" || key == "round_number" || key == "number" || key == "baris" || key == "kolom" || key == "rank" {
				typ = "integer"
			}
			v := object{"type": typ}
			if strings.HasSuffix(key, "_id") {
				v["format"] = "uuid"
			}
			if strings.Contains(key, "password") {
				v["format"] = "password"
			}
			p[key] = v
		}
		defs[name] = object{"type": "object", "additionalProperties": false, "properties": p}
	}
	defs["Scores"] = object{"type": "object", "required": []string{"scores"}, "properties": object{"scores": object{"type": "array", "minItems": 1, "items": object{"type": "object", "required": []string{"criteria_id", "score_value"}, "properties": object{"criteria_id": object{"type": "string", "format": "uuid"}, "score_value": object{"type": "string", "description": "Exact decimal string, 1–10. Send every criterion in a single transaction."}}}}}}
	defs["Envelope"] = object{"type": "object", "required": []string{"success", "message"}, "properties": object{"success": object{"type": "boolean"}, "message": object{"type": "string"}, "data": object{}, "errors": object{}}}
	defs["MediaPair"] = object{"type": "object", "required": []string{"arena_id"}, "properties": object{"arena_id": object{"type": "string", "format": "uuid"}}}
	defs["MediaMapping"] = object{"type": "object", "required": []string{"channel_id", "registration_id", "gantangan_id"}, "properties": object{"channel_id": object{"type": "string"}, "registration_id": object{"type": "string", "format": "uuid"}, "gantangan_id": object{"type": "string", "format": "uuid"}}}
	defs["MediaIngestion"] = object{"type": "object", "required": []string{"engine_id", "session_id", "round_id", "mappings"}, "properties": object{"engine_id": object{"type": "string", "format": "uuid"}, "session_id": object{"type": "string"}, "round_id": object{"type": "string", "format": "uuid"}, "mappings": object{"type": "array", "minItems": 1, "maxItems": 24, "items": object{"$ref": "#/definitions/MediaMapping"}}}}
	defs["MediaUpload"] = object{"type": "object", "required": []string{"session_id", "channel_id", "recording_id", "size_bytes", "sha256", "started_at", "ended_at"}, "properties": object{"container": object{"type": "string", "enum": []string{"mkv", "mp4"}, "default": "mp4", "description": "New desktop recordings use mkv; mp4 retained for existing recordings."}, "session_id": object{"type": "string"}, "channel_id": object{"type": "string"}, "recording_id": object{"type": "string"}, "size_bytes": object{"type": "integer", "format": "int64", "minimum": 1, "maximum": 167772160000}, "sha256": object{"type": "string", "pattern": "^[a-f0-9]{64}$"}, "started_at": object{"type": "string", "format": "date-time"}, "ended_at": object{"type": "string", "format": "date-time"}}}
	defs["MediaPart"] = object{"type": "object", "required": []string{"number", "sha256"}, "properties": object{"number": object{"type": "integer", "minimum": 1, "maximum": 10000}, "sha256": object{"type": "string", "description": "Base64 SHA-256 of this part; signed and verified by object storage."}}}
	spec := object{"swagger": "2.0", "info": object{"title": "Kicau Mania Cloud API", "version": "011", "description": "Active schema-011 routes. Opaque access tokens: 15 minutes; rotating refresh cookie: maximum 30 days. ADMIN/OPERATOR arena assignments, JURI round roster, PESERTA own reads. No self registration, Google/SMTP, legacy resources ; desktop media uses scoped engine credentials and private object storage. Master writes require expected_version; history is append-only. CRUD and workflow actions commit before success. Lists use {items,next_cursor,has_more}."}, "basePath": "/", "paths": paths, "definitions": defs, "securityDefinitions": object{"EngineAuth": object{"type": "apiKey", "in": "header", "name": "Authorization", "description": "Bearer <paired media-engine credential>; never a user access token."}, "BearerAuth": object{"type": "apiKey", "in": "header", "name": "Authorization", "description": "Bearer <opaque access token>"}}}
	b, e := json.MarshalIndent(spec, "", "  ")
	if e != nil {
		panic(e)
	}
	if len(os.Args) > 1 {
		if e = os.WriteFile(os.Args[1], append(b, '\n'), 0644); e != nil {
			panic(e)
		}
	} else {
		fmt.Println(string(b))
	}
}
func bodyName(path string) string {
	if strings.HasPrefix(path, "/api/v1/media/engines/") {
		return "MediaPair"
	}
	if path == "/api/v1/media/ingestions" {
		return "MediaIngestion"
	}
	if path == "/api/v1/engine/media/uploads" {
		return "MediaUpload"
	}
	if strings.HasPrefix(path, "/api/v1/engine/media/uploads/") && strings.HasSuffix(path, "/parts") {
		return "MediaPart"
	}

	if strings.HasSuffix(path, "/login") {
		return "Login"
	}
	if strings.HasSuffix(path, "/change-password") {
		return "Password"
	}
	if strings.HasSuffix(path, "/credentials") {
		return "Credentials"
	}
	if strings.HasSuffix(path, "/transfer") {
		return "Transfer"
	}
	if strings.Contains(path, "/scores/") {
		return "Scores"
	}
	if strings.Contains(path, "/awards") {
		return "Award"
	}
	for _, a := range []string{"start", "begin-judging", "close"} {
		if strings.HasSuffix(path, "/"+a) {
			return "Transition"
		}
	}
	if strings.Contains(path, "/rounds/") && strings.HasSuffix(path, "/cancel") {
		return "Transition"
	}
	if strings.Contains(path, "/registrations/") {
		for _, a := range []string{"verify", "reject", "disqualify", "requalify", "cancel"} {
			if strings.HasSuffix(path, "/"+a) {
				return "TicketAction"
			}
		}
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	res := parts[len(parts)-1]
	if strings.HasPrefix(res, ":") {
		res = parts[len(parts)-2]
	}
	return map[string]string{"users": "Participant", "birds": "Bird", "bird-species": "Species", "arenas": "Arena", "gantangan": "Gantangan", "events": "Event", "categories": "Category", "rounds": "Round", "criteria": "Criterion", "registrations": "Registration"}[res]
}
