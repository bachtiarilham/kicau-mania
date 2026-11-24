package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WhatsAppSender defines the contract for sending WhatsApp text & media messages.
type WhatsAppSender interface {
	SendMessage(ctx context.Context, phone, message string) error
}

// GenericWhatsAppGateway implements WhatsAppSender using HTTP REST/Webhook.
// Compatible with providers like Fonnte, Waha, Wablas, or custom webhook microservices.
type GenericWhatsAppGateway struct {
	Endpoint string
	APIKey   string
	Client   *http.Client
}

func NewGenericWhatsAppGateway(endpoint, apiKey string) *GenericWhatsAppGateway {
	return &GenericWhatsAppGateway{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *GenericWhatsAppGateway) SendMessage(ctx context.Context, phone, message string) error {
	// If no endpoint configured, acts as Mock/Development logger (graceful no-op)
	if w.Endpoint == "" {
		return nil
	}

	payload := map[string]string{
		"target":  phone,
		"phone":   phone,
		"message": message,
		"text":    message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.APIKey != "" {
		req.Header.Set("Authorization", w.APIKey)
	}

	resp, err := w.Client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp gateway unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("whatsapp gateway responded with HTTP %d", resp.StatusCode)
	}

	return nil
}
