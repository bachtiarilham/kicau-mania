package logger

import (
	"encoding/json"
	"regexp"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const RedactedPlaceholder = "[Redacted]"

var sensitiveKeys = []string{
	"password",
	"pass",
	"token",
	"authorization",
	"auth",
	"secret",
	"pin",
	"api_key",
	"cookie",
	"credential",
	"stream_key",
	"source_url",
	"database_url",
	"destination_url",
}
var credentialURL = regexp.MustCompile(`(?i)(?:https?|postgres(?:ql)?|rtsps?|rtmps?)://[^\s"'<>]+`)

type redactCore struct {
	zapcore.Core
}

func NewRedactCore(core zapcore.Core) zapcore.Core { return &redactCore{Core: core} }
func (r *redactCore) With(fields []zapcore.Field) zapcore.Core {
	return &redactCore{Core: r.Core.With(redactFields(fields))}
}
func (r *redactCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if r.Enabled(entry.Level) {
		return checked.AddCore(entry, r)
	}
	return checked
}
func (r *redactCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = credentialURL.ReplaceAllString(entry.Message, RedactedPlaceholder)
	return r.Core.Write(entry, redactFields(fields))
}
func sensitive(key string) bool {
	for _, part := range sensitiveKeys {
		if strings.Contains(strings.ToLower(key), part) {
			return true
		}
	}
	return false
}
func redactFields(fields []zapcore.Field) []zapcore.Field {
	out := make([]zapcore.Field, len(fields))
	for i, field := range fields {
		switch {
		case sensitive(field.Key):
			out[i] = zap.String(field.Key, RedactedPlaceholder)
		case field.Type == zapcore.StringType:
			field.String = credentialURL.ReplaceAllString(field.String, RedactedPlaceholder)
			out[i] = field
		case field.Type == zapcore.ErrorType:
			if err, ok := field.Interface.(error); ok {
				out[i] = zap.String(field.Key, credentialURL.ReplaceAllString(err.Error(), RedactedPlaceholder))
			} else {
				out[i] = field
			}
		case field.Type == zapcore.ReflectType || field.Type == zapcore.ObjectMarshalerType || field.Type == zapcore.ArrayMarshalerType:
			// Normalize structured fields first so nested JSON keys are redacted too.
			encoded, err := json.Marshal(field.Interface)
			var value any
			if err != nil || json.Unmarshal(encoded, &value) != nil {
				out[i] = zap.String(field.Key, RedactedPlaceholder)
			} else {
				out[i] = zap.Any(field.Key, sanitizeValue(value))
			}
		default:
			out[i] = field
		}
	}
	return out
}
func sanitizeValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if sensitive(key) {
				v[key] = RedactedPlaceholder
			} else {
				v[key] = sanitizeValue(item)
			}
		}
	case []any:
		for i := range v {
			v[i] = sanitizeValue(v[i])
		}
	case string:
		return credentialURL.ReplaceAllString(v, RedactedPlaceholder)
	}
	return value
}
