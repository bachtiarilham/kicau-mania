// Package request handles JSON syntax, pagination syntax and authenticated context extraction.
package request

import (
	"bufio"
	"encoding/json"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/middleware"
	"kicau.cloud/control/internal/delivery/rest/response"
)

func Bind(c *gin.Context, out any) bool {
	if c.ContentType() != "application/json" {
		response.BadRequest(c, "application/json is required", nil)
		return false
	}
	reader := bufio.NewReader(c.Request.Body)
	for {
		b, e := reader.ReadByte()
		if e != nil {
			response.BadRequest(c, "A JSON object is required", nil)
			return false
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		if b != '{' {
			response.BadRequest(c, "A JSON object is required", nil)
			return false
		}
		_ = reader.UnreadByte()
		break
	}
	d := json.NewDecoder(reader)
	d.UseNumber()
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		response.BadRequest(c, "Invalid request syntax", nil)
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		response.BadRequest(c, "Only one JSON value is allowed", nil)
		return false
	}
	normalizeIDs(reflect.ValueOf(out), "")
	return true
}
func Actor(c *gin.Context) string { id, _ := middleware.User(c.Request.Context()); return id.UserID }
func Page(c *gin.Context) (string, int, bool) {
	n, e := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if e != nil {
		response.BadRequest(c, "Invalid limit syntax", nil)
		return "", 0, false
	}
	return c.Query("cursor"), n, true
}
func PageData(rows []model.Record, limit int) response.CursorPage[model.Record] {
	p := response.CursorPage[model.Record]{Items: rows}
	if len(rows) > limit && limit > 0 {
		p.Items = rows[:limit]
		p.HasMore = true
		p.NextCursor = model.Text(p.Items[len(p.Items)-1], "id")
	}
	return p
}

// UUID canonicalization is transport parsing; passwords, tokens and labels are untouched.
func normalizeIDs(v reflect.Value, key string) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			normalizeIDs(v.Elem(), key)
		}
	case reflect.Interface:
		if !v.IsNil() {
			copy := reflect.New(v.Elem().Type()).Elem()
			copy.Set(v.Elem())
			normalizeIDs(copy, key)
			if v.CanSet() {
				v.Set(copy)
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			tag := strings.Split(field.Tag.Get("json"), ",")[0]
			if tag == "-" || !field.IsExported() {
				continue
			}
			normalizeIDs(v.Field(i), tag)
		}
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return
		}
		for _, k := range v.MapKeys() {
			value := v.MapIndex(k)
			copy := reflect.New(value.Type()).Elem()
			copy.Set(value)
			normalizeIDs(copy, k.String())
			v.SetMapIndex(k, copy)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			normalizeIDs(v.Index(i), key)
		}
	case reflect.String:
		if v.CanSet() && (key == "id" || strings.HasSuffix(key, "_id")) && model.UUID(v.String()) {
			v.SetString(strings.ToLower(v.String()))
		}
	}
}
