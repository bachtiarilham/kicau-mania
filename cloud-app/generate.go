// Package cloudapp holds reproducible developer commands; the server entrypoint is cmd/api.
//
// @title Kicau Mania Cloud API
// @version 1.0.0
// @description REST cloud-app. Urutan: login, persiapan/roster, registrasi, pembayaran, ronde, penilaian/revisi dan hasil. Try it out mengirim request nyata. Swagger JSON dihasilkan otomatis dari anotasi dan DTO; jangan diedit manual.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Masukkan Bearer diikuti spasi dan data.access_token dari login. Token opaque berlaku 900 detik. Ganti token saat beralih role.
package cloudapp

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo generate.go --dir . --exclude runtime,docs --parseInternal --parseFuncBody --output docs/generated --outputTypes go,json --quiet
