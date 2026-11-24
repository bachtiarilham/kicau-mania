package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kicau.cloud/control/internal/media/service"
)

func TestGenericWhatsAppGateway_SendMessage(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	gw := service.NewGenericWhatsAppGateway(server.URL, "Bearer secret-token")
	err := gw.SendMessage(context.Background(), "081234567890", "Halo test kicau")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received["phone"] != "081234567890" || received["message"] != "Halo test kicau" {
		t.Fatalf("unexpected payload received: %v", received)
	}
}

func TestGenericWhatsAppGateway_NoEndpointGraceful(t *testing.T) {
	gw := service.NewGenericWhatsAppGateway("", "")
	err := gw.SendMessage(context.Background(), "081234567890", "Halo test kicau")
	if err != nil {
		t.Fatalf("expected nil error on empty endpoint, got: %v", err)
	}
}
