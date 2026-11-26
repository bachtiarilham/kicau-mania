package model

import (
	"encoding/json"
	"strings"

	"kicau.cloud/control/internal/apperror"
)

func Validate(r Record, required, ids, positive, amounts string, create bool) error {
	for _, k := range strings.Fields(required) {
		v, ok := r[k]
		text, isText := v.(string)
		if (create || ok) && (v == nil || isText && strings.TrimSpace(text) == "") {
			return apperror.Invalid("Required field: " + k)
		}
	}
	for _, k := range strings.Fields(ids) {
		if v, ok := r[k]; ok && v != nil {
			if !NumericID(Text(r, k)) {
				return apperror.Invalid("Invalid identifier: " + k)
			}
		}
	}
	for _, k := range strings.Fields(positive) {
		if _, ok := r[k]; ok {
			v, valid := r[k].(json.Number)
			if !valid {
				return apperror.ErrInvalid
			}
			n, e := v.Int64()
			if e != nil || n < 1 || n > 100000 {
				return apperror.ErrInvalid
			}
		}
	}
	for _, k := range strings.Fields(amounts) {
		if v, ok := r[k]; ok {
			n, e := Cents(v)
			if e != nil || n < 0 || n > 999999999999 {
				return apperror.Invalid("Invalid amount: " + k)
			}
		}
	}
	for k, v := range r {
		switch v.(type) {
		case nil, string, bool, json.Number:
		default:
			return apperror.Invalid("Invalid field type: " + k)
		}
		if text, ok := v.(string); ok && (len(text) > 4096 || strings.ContainsAny(text, "\x00\r\n")) {
			return apperror.Invalid("Invalid field: " + k)
		}
	}
	return nil
}

func Strings(r Record, keys string) error {
	for _, k := range strings.Fields(keys) {
		if v, ok := r[k]; ok && v != nil {
			if _, ok := v.(string); !ok {
				return apperror.Invalid("Invalid text field: " + k)
			}
		}
	}
	return nil
}
