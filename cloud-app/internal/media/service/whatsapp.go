package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"kicau.cloud/control/internal/infra/resilience"
)

// WhatsAppSender defines the contract for sending WhatsApp text & media messages.
type WhatsAppSender interface {
	SendMessage(ctx context.Context, phone, message string) error
}

// GenericWhatsAppGateway implements WhatsAppSender using HTTP REST/Webhook with Circuit Breaker resilience.
// Compatible with providers like Fonnte, Waha, Wablas, or custom webhook microservices.
type GenericWhatsAppGateway struct {
	Endpoint string
	APIKey   string
	Client   *http.Client
	CB       *resilience.CircuitBreaker
}

func NewGenericWhatsAppGateway(endpoint, apiKey string) *GenericWhatsAppGateway {
	return &GenericWhatsAppGateway{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Client:   &http.Client{Timeout: 10 * time.Second},
		CB:       resilience.NewCircuitBreaker("whatsapp-gateway", 5, 10*time.Second, 30*time.Second),
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

	_, execErr := w.CB.Execute(func() (any, error) {
		resp, err := w.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("whatsapp gateway unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("whatsapp gateway server error: HTTP %d", resp.StatusCode)
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("whatsapp gateway responded with HTTP %d", resp.StatusCode)
		}

		return nil, nil
	})

	if execErr != nil {
		if errors.Is(execErr, resilience.ErrCircuitBreakerOpen) {
			return fmt.Errorf("%w: whatsapp gateway is temporarily unavailable", execErr)
		}
		return execErr
	}

	return nil
}
