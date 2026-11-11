// Package localapp holds reproducible documentation commands; cmd/api runs the engine.
//
// @title Kicau Mania Local Media Engine API
// @version 1.0.0
// @description Backend media LAN/cloud. Mulai dari setup/bootstrap (mode lan, akses localhost), checks, jobs lalu media/sessions. Endpoint media tersedia setelah engine dikonfigurasi. Try it out mengirim request nyata. Swagger tidak menampilkan video atau grafik. WHEP memakai SDP; WebSocket memerlukan client tersendiri. Jangan menaruh API_TOKEN engine pada frontend pengguna.
// @BasePath /
// @securityDefinitions.apikey OperatorToken
// @in header
// @name Authorization
// @description Masukkan Bearer diikuti API_TOKEN untuk kontrol layanan. Alternatif khusus mode LAN dari localhost: LocalControl.
// @securityDefinitions.apikey LocalControl
// @in header
// @name X-Local-Control
// @description Mode LAN dari localhost saja. Ambil data.nonce melalui GET /api/v1/setup/bootstrap. Nonce berubah saat aplikasi restart. Setup tidak menerima OperatorToken sebagai pengganti nonce.
// @securityDefinitions.apikey PlaybackTicket
// @in header
// @name Authorization
// @description Mode cloud: Bearer diikuti tiket playback cloud, bukan API_TOKEN. Mode LAN: tidak memerlukan tiket, tetap dibatasi subnet dan sesi aktif.
package localapp

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo generate.go --dir . --exclude runtime,bin,docs --parseInternal --output docs/generated --outputTypes go,json --quiet
