package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"image"
	"image/png"
	"io"
	"kicau.cloud/control/config"
	"kicau.cloud/control/db/queries"
	"kicau.cloud/control/internal/delivery"
	redisService "kicau.cloud/control/internal/login/service"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// Opt-in: creates and drops ONLY a uniquely named test database on the explicitly
// supplied test cluster. It never reads DATABASE_URL or the operational .env.
func TestSchema010BusinessFlows(t *testing.T) {
	base := os.Getenv("KICAU_TEST_POSTGRES_URL")
	redisURL := os.Getenv("KICAU_TEST_REDIS_URL")
	if base == "" || redisURL == "" {
		t.Skip("Set isolated KICAU_TEST_POSTGRES_URL and KICAU_TEST_REDIS_URL")
	}
	ctx := context.Background()
	root, e := pgx.Connect(ctx, base)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close(ctx)
	dbname := fmt.Sprintf("kicau_test_%d", time.Now().UnixNano())
	if _, e = root.Exec(ctx, "CREATE DATABASE "+dbname); e != nil {
		t.Fatal(e)
	}
	defer func() { _, _ = root.Exec(ctx, "DROP DATABASE "+dbname+" WITH (FORCE)") }()
	cfg, e := pgx.ParseConfig(base)
	if e != nil {
		t.Fatal(e)
	}
	cfg.Database = dbname
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db, e := pgx.ConnectConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close(ctx)
	scaffold := `DO $$ DECLARE n text; BEGIN FOREACH n IN ARRAY ARRAY['kicau_cloud_app','kicau_cloud_reader','kicau_cloud_admin_reader','kicau_cloud_recap_maintainer'] LOOP IF NOT EXISTS(SELECT FROM pg_roles WHERE rolname=n) THEN EXECUTE format('CREATE ROLE %I NOLOGIN',n); END IF; END LOOP; END $$;
 CREATE SCHEMA iam;CREATE SCHEMA asset;CREATE SCHEMA competition;CREATE SCHEMA judging;CREATE SCHEMA media;CREATE SCHEMA member;CREATE SCHEMA cloud_read;CREATE TABLE judging.participant_assessments(id int);CREATE TABLE cloud_read.refresh_status(id int);`
	if _, e = db.Exec(ctx, scaffold); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"008_simplified_erd_reset.up.sql", "009_simplified_cloud_read.up.sql", "010_master_history_and_soft_delete.up.sql"} {
		b, e := os.ReadFile(filepath.Join("../../db/migrations", name))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, "BEGIN;"+string(b)+"COMMIT;"); e != nil {
			t.Fatalf("%s: %v", name, e)
		}
	}
	// Prepare every query as the actual application role; run rollback scenarios.
	var prepared strings.Builder
	for _, name := range queries.Names() {
		if strings.Contains(queries.Get(name), "REFRESH MATERIALIZED VIEW") {
			continue
		}
		fmt.Fprintf(&prepared, "PREPARE %s AS %s\n", strings.ToLower(name), queries.Get(name))
	}
	scenario, e := os.ReadFile("../../db/verification/010_business_flows.sql")
	if e != nil {
		t.Fatal(e)
	}
	sql := strings.Replace(string(scenario), "SET LOCAL ROLE kicau_cloud_app;", "SET LOCAL ROLE kicau_cloud_app;\n"+prepared.String(), 1)
	sql = regexp.MustCompile(`(?m)^SELECT set_config\('flow\.\w+',:'\w+',true\);\r?$`).ReplaceAllString(sql, "")
	sql = regexp.MustCompile(`(?m)^SELECT id AS (\w+) FROM (.*?) \\gset\r?$`).ReplaceAllString(sql, "SELECT set_config('flow.$1',id::text,true) FROM $2;")
	sql = regexp.MustCompile(`:'(\w+)'`).ReplaceAllString(sql, "current_setting('flow.$1')::uuid")
	if _, e = db.Exec(ctx, sql); e != nil {
		t.Fatalf("named SQL/scenarios: %v", e)
	}
	dbURL, _ := url.Parse(base)
	dbURL.Path = "/" + dbname
	options := dbURL.Query()
	options.Set("options", "-c role=kicau_cloud_app")
	options.Set("default_query_exec_mode", "simple_protocol")
	dbURL.RawQuery = strings.ReplaceAll(options.Encode(), "+", "%20")
	c, e := New(ctx, config.Config{DatabaseURL: dbURL.String(), RedisURL: redisURL, DBMaxConns: 8, DBConnectTimeout: 3 * time.Second, DBHealthTimeout: 3 * time.Second, CookieName: "kicau_refresh", AllowedOrigins: []string{"http://web.test"}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	var dbRole string
	if e = c.Database.QueryRow(ctx, "SELECT current_user").Scan(&dbRole); e != nil || dbRole != "kicau_cloud_app" {
		t.Fatalf("API must use application grants: %s %v", dbRole, e)
	}
	if e = c.Workflow.Bootstrap(ctx, "Admin", "admin@example.test", "081200000001", "test-password-admin-010"); e != nil {
		t.Fatalf("bootstrap: %v", e)
	}
	engine := delivery.Router(delivery.Options{AllowedOrigins: []string{"http://web.test"}, RequestTimeout: 15 * time.Second}, c.Handlers)
	server := httptest.NewServer(engine)
	defer server.Close()
	apiClient := &http.Client{Transport: handlerTransport{handler: engine}}
	docResponse, e := apiClient.Get(server.URL + "/api/docs/doc.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	e = json.NewDecoder(docResponse.Body).Decode(&doc)
	docResponse.Body.Close()
	if e != nil {
		t.Fatal("invalid Swagger", e)
	}
	pathParams := regexp.MustCompile(`:(\w+)`)
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/") {
			path := pathParams.ReplaceAllString(route.Path, "{$1}")
			if doc.Paths[path][strings.ToLower(route.Method)] == nil {
				t.Fatalf("Swagger missing active route %s %s", route.Method, path)
			}
		}
	}
	for path := range doc.Paths {
		for _, old := range []string{"/owners", "/engines", "/devices", "/media", "/auth/register", "/auth/google"} {
			if strings.Contains(path, old+"/") || strings.HasSuffix(path, old) {
				t.Fatal("Swagger advertises obsolete route", path)
			}
		}
	}
	token := ""
	refresh := ""
	call := func(method, path string, body any, status int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://web.test")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if refresh != "" {
			req.AddCookie(&http.Cookie{Name: "kicau_refresh", Value: refresh})
		}
		r, e := apiClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		if r.StatusCode != status {
			t.Fatalf("%s %s got %d want %d: %s", method, path, r.StatusCode, status, raw)
		}
		for _, cookie := range r.Cookies() {
			if cookie.Name == "kicau_refresh" {
				refresh = cookie.Value
			}
		}
		v := map[string]any{}
		_ = json.Unmarshal(raw, &v)
		data, _ := v["data"].(map[string]any)
		return data
	}
	call("POST", "/auth/login", map[string]any{"email": "admin@example.test", "password": "incorrect-password", "device_id": "test-device-01"}, 401)
	login := call("POST", "/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password-admin-010", "device_id": "test-device-01"}, 200)
	token = login["access_token"].(string)
	digest := sha256.Sum256([]byte(token))
	key := "km:a:" + hex.EncodeToString(digest[:])
	ttl := c.Redis.Client.PTTL(ctx, key).Val()
	if ttl > 15*time.Minute || ttl < 14*time.Minute {
		t.Fatal("invalid access TTL", ttl)
	}
	stored := c.Redis.Client.Get(ctx, key).Val()
	if strings.Contains(stored, token) || strings.Contains(stored, refresh) {
		t.Fatal("raw token persisted in Redis")
	}
	call("GET", "/auth/me", nil, 200)
	stale := refresh
	login = call("POST", "/auth/refresh", map[string]any{}, 200)
	token = login["access_token"].(string)
	rotated := refresh
	refresh = stale
	call("POST", "/auth/refresh", map[string]any{}, 401)
	refresh = rotated
	arena := call("POST", "/arenas", map[string]any{"name": "Arena test"}, 200)
	a := arena["id"].(string)
	user := call("POST", "/users", map[string]any{"name": "Peserta", "phone": "081200000002", "email": "owner@example.test", "is_eo": true}, 200)
	uid := user["id"].(string)
	noOp := call("PATCH", "/users/"+uid, map[string]any{"name": "Peserta", "expected_version": 1}, 200)
	if noOp["version"] != float64(1) {
		t.Fatal("no-op changed version")
	}
	user = call("PATCH", "/users/"+uid, map[string]any{"name": "Peserta baru", "expected_version": 1}, 200)
	call("PATCH", "/users/"+uid, map[string]any{"name": "stale editor", "expected_version": 1}, 409)
	species := call("POST", "/bird-species", map[string]any{"name": "Murai"}, 200)
	bird := call("POST", "/birds", map[string]any{"name": "Juara", "owner_id": uid, "bird_species_id": species["id"]}, 200)
	event := call("POST", "/arenas/"+a+"/events", map[string]any{"owner_id": uid, "title": "Event", "description": "", "event_date": "2026-11-15", "status": "SCHEDULED"}, 200)
	round := call("POST", "/arenas/"+a+"/rounds", map[string]any{"event_id": event["id"], "bird_species_id": species["id"], "title": "Ronde", "round_number": 1}, 200)
	gantangan := call("POST", "/arenas/"+a+"/gantangan", map[string]any{"number": 1, "baris": 1, "kolom": 1}, 200)
	ticketInput := map[string]any{"user_id": uid, "arena_id": a, "round_id": round["id"], "bird_id": bird["id"], "gantangan_id": gantangan["id"], "entry_fee": "150000.50", "payment_method": "CASH"}
	ticket := call("POST", "/registrations", ticketInput, 200)
	if frontend := os.Getenv("KICAU_TEST_FRONTEND_DIR"); frontend != "" {
		cmd := exec.CommandContext(ctx, "node", "tests/live-contract.mjs")
		cmd.Dir = frontend
		cmd.Env = append(os.Environ(), "KICAU_TEST_API_URL="+server.URL, "KICAU_TEST_ACCESS_TOKEN="+token)
		output, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("frontend live contract: %v\n%s", e, output)
		}
		t.Log(strings.TrimSpace(string(output)))
	}
	tid := ticket["id"].(string)
	call("POST", "/registrations", ticketInput, 409)
	call("POST", "/registrations/"+tid+"/verify?arena_id="+a, map[string]any{}, 200)
	call("PUT", "/registrations/"+tid, ticketInput, 409)
	call("POST", "/registrations/"+tid+"/disqualify?arena_id="+a, map[string]any{"reason": ""}, 400)
	dq := call("POST", "/registrations/"+tid+"/disqualify?arena_id="+a, map[string]any{"reason": "Melanggar aturan"}, 200)
	if dq["payment_status"] != "VERIFIED" {
		t.Fatal("DQ changed payment")
	}
	call("POST", "/registrations/"+tid+"/requalify?arena_id="+a, map[string]any{}, 200)
	// Ticket edits remain possible before verification; wrong species/arena fail.
	round2 := call("POST", "/arenas/"+a+"/rounds", map[string]any{"event_id": event["id"], "bird_species_id": species["id"], "title": "Ronde kedua", "round_number": 2}, 200)
	transferInput := map[string]any{}
	for k, v := range ticketInput {
		transferInput[k] = v
	}
	transferInput["round_id"] = round2["id"]
	transferInput["payment_method"] = "TRANSFER"
	transferTicket := call("POST", "/registrations", transferInput, 200)["id"].(string)
	transferInput["entry_fee"] = "175000.25"
	call("PUT", "/registrations/"+transferTicket, transferInput, 200)
	wrongSpecies := call("POST", "/bird-species", map[string]any{"name": "Kenari"}, 200)
	wrongBird := call("POST", "/birds", map[string]any{"name": "Kenari", "owner_id": uid, "bird_species_id": wrongSpecies["id"]}, 200)
	badInput := map[string]any{}
	for k, v := range transferInput {
		badInput[k] = v
	}
	badInput["bird_id"] = wrongBird["id"]
	call("POST", "/registrations", badInput, 400)
	call("POST", "/registrations/"+transferTicket+"/verify?arena_id="+a, map[string]any{}, 409)
	upload := func(b []byte, want int) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "proof.png")
		_, _ = part.Write(b)
		_ = writer.Close()
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/registrations/"+transferTicket+"/proof?arena_id="+a, &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		r, e := apiClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		if r.StatusCode != want {
			b, _ := io.ReadAll(r.Body)
			t.Fatalf("proof got %d: %s", r.StatusCode, b)
		}
	}
	upload([]byte("fake png"), 400)
	upload(bytes.Repeat([]byte{'x'}, (5<<20)+1), 400)
	var pngBytes bytes.Buffer
	_ = png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	upload(pngBytes.Bytes(), 200)
	proofResponse, e := http.NewRequest("GET", server.URL+"/api/v1/registrations/"+transferTicket+"/proof?arena_id="+a, nil)
	if e != nil {
		t.Fatal(e)
	}
	proofResponse.Header.Set("Authorization", "Bearer "+token)
	pr, e := apiClient.Do(proofResponse)
	if e != nil {
		t.Fatal(e)
	}
	if pr.StatusCode != 200 || pr.Header.Get("Content-Type") != "image/png" {
		t.Fatal("authorized proof failed")
	}
	pr.Body.Close()
	call("POST", "/registrations/"+transferTicket+"/reject?arena_id="+a, map[string]any{}, 200)
	call("POST", "/registrations/"+transferTicket+"/verify?arena_id="+a, map[string]any{}, 200)
	call("POST", "/registrations/"+transferTicket+"/cancel?arena_id="+a, map[string]any{}, 200)
	adminToken, adminRefresh := token, refresh
	adminID := login["user"].(map[string]any)["id"].(string)
	call("DELETE", "/arenas/"+a+"/staff/"+adminID+"/ADMIN", nil, 409)
	otherArena := call("POST", "/arenas", map[string]any{"name": "Arena lain"}, 200)["id"].(string)
	judgeID := call("POST", "/users", map[string]any{"name": "Juri", "phone": "081200000003", "email": "judge@example.test"}, 200)["id"].(string)
	opID := call("POST", "/users", map[string]any{"name": "Operator", "phone": "081200000004", "email": "operator@example.test"}, 200)["id"].(string)
	for _, pair := range []struct{ id, role string }{{judgeID, "JURI"}, {opID, "OPERATOR"}} {
		call("POST", "/arenas/"+a+"/staff/"+pair.id+"/"+pair.role, map[string]any{}, 200)
		call("POST", "/arenas/"+a+"/staff/"+pair.id+"/credentials", map[string]any{"password": "test-password-staff-010"}, 200)
	}
	opLogin := call("POST", "/auth/login", map[string]any{"email": "operator@example.test", "password": "test-password-staff-010", "device_id": "operator-device-01"}, 200)
	token = opLogin["access_token"].(string)
	call("GET", "/arenas/"+otherArena+"/events", nil, 403)
	call("GET", "/registrations?arena_id="+otherArena, nil, 403)
	call("POST", "/arenas", map[string]any{"name": "forbidden"}, 403)
	call("POST", "/arenas/"+a+"/staff/"+opID+"/ADMIN", map[string]any{}, 403)
	call("PATCH", "/users/"+judgeID, map[string]any{"name": "forbidden", "expected_version": 1}, 403)
	call("POST", "/arenas/"+a+"/staff/"+opID+"/credentials", map[string]any{"password": "not-allowed-password"}, 403)
	token, refresh = adminToken, adminRefresh
	baseRound := "/arenas/" + a + "/rounds/" + round["id"].(string)
	criterion1 := call("POST", "/arenas/"+a+"/criteria", map[string]any{"criteria_name": "Irama"}, 200)["id"].(string)
	criterion2 := call("POST", "/arenas/"+a+"/criteria", map[string]any{"criteria_name": "Volume"}, 200)["id"].(string)
	call("POST", baseRound+"/start", map[string]any{"expected_status": "PREPARED"}, 409)
	judgeLogin := call("POST", "/auth/login", map[string]any{"email": "judge@example.test", "password": "test-password-staff-010", "device_id": "judge-device-01"}, 200)
	judgeToken, judgeRefresh := judgeLogin["access_token"].(string), refresh
	token = judgeToken
	call("PUT", baseRound+"/scores/"+tid, map[string]any{"scores": []any{}}, 400)
	call("GET", baseRound+"/participants", nil, 403)
	token, refresh = adminToken, adminRefresh
	call("POST", baseRound+"/judges/"+judgeID, map[string]any{}, 200)
	call("POST", baseRound+"/start", map[string]any{"expected_status": "PREPARED"}, 200)
	call("PATCH", "/arenas/"+a+"/criteria/"+criterion1, map[string]any{"criteria_name": "changed"}, 409)
	call("POST", baseRound+"/begin-judging", map[string]any{"expected_status": "RUNNING"}, 200)
	call("POST", baseRound+"/close", map[string]any{"expected_status": "JUDGING"}, 409)
	values := func(a, b string) map[string]any {
		return map[string]any{"scores": []any{map[string]any{"criteria_id": criterion1, "score_value": a}, map[string]any{"criteria_id": criterion2, "score_value": b}}}
	}
	call("PUT", baseRound+"/scores/"+tid, values("8.5", "9.25"), 403)
	token, refresh = judgeToken, judgeRefresh
	call("PUT", baseRound+"/scores/"+tid, values("8.5", "99"), 400)
	var count int
	if e = db.QueryRow(ctx, "SELECT count(*) FROM judging.round_scores").Scan(&count); e != nil || count != 0 {
		t.Fatalf("partial score package persisted: %d %v", count, e)
	}
	call("PUT", baseRound+"/scores/"+tid, values("8.5", "9.25"), 200)
	call("PUT", baseRound+"/scores/"+tid, values("9.25", "9.75"), 200)
	var minimum string
	if e = db.QueryRow(ctx, "SELECT min(score_value)::text FROM judging.round_scores").Scan(&minimum); e != nil || minimum != "9.25" {
		t.Fatalf("lost decimal revision: %s %v", minimum, e)
	}
	token, refresh = adminToken, adminRefresh
	call("POST", "/registrations/"+tid+"/disqualify?arena_id="+a, map[string]any{"reason": "DQ saat penjurian"}, 200)
	token = judgeToken
	call("PUT", baseRound+"/scores/"+tid, values("5", "5"), 409)
	token = adminToken
	call("POST", "/registrations/"+tid+"/requalify?arena_id="+a, map[string]any{}, 200)
	// Both handlers use the same round lock: closure cannot observe half a package.
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, request := range []struct {
		method, path, auth string
		body               any
	}{{"POST", baseRound + "/close", adminToken, map[string]any{"expected_status": "JUDGING"}}, {"PUT", baseRound + "/scores/" + tid, judgeToken, values("9.5", "9.75")}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, _ := json.Marshal(request.body)
			req, _ := http.NewRequest(request.method, server.URL+"/api/v1"+request.path, bytes.NewReader(b))
			req.Header.Set("Authorization", "Bearer "+request.auth)
			req.Header.Set("Content-Type", "application/json")
			r, e := apiClient.Do(req)
			if e != nil {
				statuses <- 0
				return
			}
			r.Body.Close()
			statuses <- r.StatusCode
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 && status != 409 {
			t.Fatalf("concurrent close/score status %d", status)
		}
	}
	call("POST", "/registrations/"+tid+"/disqualify?arena_id="+a, map[string]any{"reason": "too late"}, 409)
	call("POST", "/registrations/"+tid+"/requalify?arena_id="+a, map[string]any{}, 409)
	call("POST", "/birds/"+bird["id"].(string)+"/awards?arena_id="+a, map[string]any{"award_name": "Juara internal", "registration_id": tid, "awarded_at": "2026-11-15", "rank": 1}, 200)
	call("POST", "/birds/"+bird["id"].(string)+"/transfer", map[string]any{"owner_id": opID, "expected_version": 1, "notes": "Terjual"}, 200)
	call("POST", "/birds/"+bird["id"].(string)+"/transfer", map[string]any{"owner_id": opID, "expected_version": 2, "notes": "Identik"}, 200)
	var version int
	if e = db.QueryRow(ctx, "SELECT version FROM asset.birds WHERE id=$1", bird["id"]).Scan(&version); e != nil || version != 2 {
		t.Fatal("no-op transfer changed version", e)
	}
	call("DELETE", "/birds/"+bird["id"].(string)+"?expected_version=2", nil, 200)
	call("GET", "/birds/"+bird["id"].(string), nil, 404)
	call("GET", "/birds/"+bird["id"].(string)+"/history", nil, 200)
	if _, e = db.Exec(ctx, "SET ROLE kicau_cloud_app; UPDATE asset.bird_master_history SET version=999"); e == nil {
		t.Fatal("history was writable")
	}
	_, _ = db.Exec(ctx, "RESET ROLE")
	for _, device := range []string{"test-device-02", "test-device-03"} {
		call("POST", "/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password-admin-010", "device_id": device}, 200)
	}
	call("POST", "/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password-admin-010", "device_id": "test-device-04"}, 409)
	// Reusing a device rotates its token pair without consuming a fourth slot.
	login = call("POST", "/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password-admin-010", "device_id": "test-device-01"}, 200)
	token = login["access_token"].(string)
	call("POST", "/auth/logout", map[string]any{}, 200)
	call("GET", "/auth/me", nil, 401)
	// Exactly one concurrent refresh of the same digest is accepted.
	var refreshWG sync.WaitGroup
	refreshStatuses := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		refreshWG.Add(1)
		go func() { defer refreshWG.Done(); _, e := c.Auth.Refresh(ctx, judgeRefresh); refreshStatuses <- e == nil }()
	}
	refreshWG.Wait()
	close(refreshStatuses)
	successes := 0
	for ok := range refreshStatuses {
		if ok {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("atomic refresh accepted %d requests", successes)
	}
	login = call("POST", "/auth/login", map[string]any{"email": "judge@example.test", "password": "test-password-staff-010", "device_id": "judge-device-01"}, 200)
	token = login["access_token"].(string)
	call("POST", "/auth/change-password", map[string]any{"old_password": "incorrect-password", "new_password": "new-password-staff-010"}, 401)
	call("GET", "/auth/me", nil, 200)
	call("POST", "/auth/change-password", map[string]any{"old_password": "test-password-staff-010", "new_password": "new-password-staff-010"}, 200)
	call("GET", "/auth/me", nil, 401)
	login = call("POST", "/auth/login", map[string]any{"email": "judge@example.test", "password": "new-password-staff-010", "device_id": "judge-device-01"}, 200)
	token = login["access_token"].(string)
	var sessionID string
	if e = db.QueryRow(ctx, "SELECT id::text FROM iam.user_sessions WHERE user_id=$1 AND revoked_at IS NULL", judgeID).Scan(&sessionID); e != nil {
		t.Fatal(e)
	}
	call("POST", "/auth/sessions/"+sessionID+"/revoke", map[string]any{}, 200)
	call("GET", "/auth/me", nil, 401)
	login = call("POST", "/auth/login", map[string]any{"email": "judge@example.test", "password": "new-password-staff-010", "device_id": "judge-device-01"}, 200)
	judgeToken, judgeRefresh = login["access_token"].(string), refresh
	ready, e := apiClient.Get(server.URL + "/readyz")
	if e != nil {
		t.Fatal(e)
	}
	if ready.StatusCode != 200 {
		t.Fatal("readiness did not check schema/Redis")
	}
	ready.Body.Close()
	// Redis failures remain 503 even for otherwise-valid SQL sessions.
	token, refresh = judgeToken, judgeRefresh
	c.Redis.Close()
	call("GET", "/auth/me", nil, 503)
	call("POST", "/auth/refresh", map[string]any{}, 503)
	ready, e = apiClient.Get(server.URL + "/readyz")
	if e != nil {
		t.Fatal(e)
	}
	if ready.StatusCode != 503 {
		t.Fatal("readiness ignored Redis failure")
	}
	ready.Body.Close()
	// Cleanup only token keys belonging to sessions in this disposable database.
	cleanup, e := redisService.New(redisURL)
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup.Close()
	rows, e := db.Query(ctx, "SELECT id::text FROM iam.user_sessions")
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			t.Fatal(e)
		}
		s, e := cleanup.Resolve(ctx, "s", id)
		if e == nil {
			if e = cleanup.Delete(ctx, s); e != nil {
				t.Fatal(e)
			}
		}
	}
}

// Exercise HTTP middleware/handlers with real SQL/Redis without coupling every
// assertion to the host's loopback socket stack. The optional frontend contract
// check still uses the actual httptest listener.
type handlerTransport struct{ handler http.Handler }

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.RequestURI = req.URL.RequestURI()
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, req)
	if req.Body != nil {
		req.Body.Close()
	}
	return w.Result(), nil
}
