package usecase_test

import (
	"context"
	"testing"

	core "kicau.cloud/control/internal/core/model"
	mediaModel "kicau.cloud/control/internal/media/model"
	"kicau.cloud/control/internal/media/usecase"
)

type mockData struct {
	oneFn  func(context.Context, string, ...any) (core.Record, error)
	manyFn func(context.Context, string, ...any) ([]core.Record, error)
	execFn func(context.Context, string, ...any) error
}

func (m *mockData) One(ctx context.Context, name string, args ...any) (core.Record, error) {
	if m.oneFn != nil {
		return m.oneFn(ctx, name, args...)
	}
	return core.Record{}, nil
}

func (m *mockData) Many(ctx context.Context, name string, args ...any) ([]core.Record, error) {
	if m.manyFn != nil {
		return m.manyFn(ctx, name, args...)
	}
	return nil, nil
}

func (m *mockData) Exec(ctx context.Context, name string, args ...any) error {
	if m.execFn != nil {
		return m.execFn(ctx, name, args...)
	}
	return nil
}

func (m *mockData) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type mockACL struct{}

func (m *mockACL) Require(context.Context, string, string, ...string) error {
	return nil
}

type mockObjects struct {
	downloadURL string
}

func (m *mockObjects) Begin(context.Context, string, string) (string, error) { return "", nil }
func (m *mockObjects) Sign(context.Context, string, string, mediaModel.SignPart) (mediaModel.Signed, error) {
	return mediaModel.Signed{}, nil
}
func (m *mockObjects) Complete(context.Context, string, string, []mediaModel.Part) error {
	return nil
}
func (m *mockObjects) Exists(context.Context, string, int64, string) (bool, error) {
	return true, nil
}
func (m *mockObjects) Parts(context.Context, string, string) ([]mediaModel.Part, error) {
	return nil, nil
}
func (m *mockObjects) Download(context.Context, string) (string, error) {
	return m.downloadURL, nil
}

type mockWhatsApp struct {
	sentTo   string
	sentText string
}

func (m *mockWhatsApp) SendMessage(ctx context.Context, phone, message string) error {
	m.sentTo = phone
	m.sentText = message
	return nil
}

func TestUseCase_DeliverWhatsApp(t *testing.T) {
	ctx := context.Background()
	assetID := "1"
	publicCode := "018f0a00-1111-7000-8000-000000000001"
	regID := "018f0a00-2222-7000-8000-000000000002"
	arenaID := "018f0a00-3333-7000-8000-000000000003"
	actorID := "018f0a00-4444-7000-8000-000000000004"

	db := &mockData{
		oneFn: func(ctx context.Context, name string, args ...any) (core.Record, error) {
			if name == "DesktopAssetRead" {
				return core.Record{
					"public_code": publicCode, "id": assetID,
					"state":           "UPLOADED",
					"registration_id": regID,
					"arena_id":        arenaID,
					"user_id":         actorID,
					"channel_id":      "cam-05",
					"object_key":      "arenas/test/video.mkv",
					"user_phone":      "081298765432",
					"snapshot": map[string]any{
						"bird_name":       "Halilintar",
						"registrant_name": "Budi Santoso",
					},
				}, nil
			}
			return nil, nil
		},
	}

	wa := &mockWhatsApp{}
	obj := &mockObjects{downloadURL: "https://storage.test/presigned-video.mp4"}

	uc := &usecase.UseCase{
		DB:       db,
		ACL:      &mockACL{},
		Objects:  obj,
		WhatsApp: wa,
	}

	res, err := uc.DeliverWhatsApp(ctx, actorID, assetID, "https://kicau.app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "SENT" {
		t.Errorf("expected status SENT, got %s", res.Status)
	}
	if res.RecipientPhone != "081298765432" {
		t.Errorf("expected phone 081298765432, got %s", res.RecipientPhone)
	}
	if wa.sentTo != "081298765432" {
		t.Errorf("expected WA sent to 081298765432, got %s", wa.sentTo)
	}
	if res.StreamingURL != "https://kicau.app/v/"+publicCode {
		t.Errorf("expected streaming url https://kicau.app/v/%s, got %s", assetID, res.StreamingURL)
	}
}

func TestUseCase_PublicWatch(t *testing.T) {
	ctx := context.Background()
	assetID := "018f0a00-1111-7000-8000-000000000001"

	db := &mockData{
		oneFn: func(ctx context.Context, name string, args ...any) (core.Record, error) {
			if name == "DesktopPublicWatch" {
				return core.Record{
					"id":         assetID,
					"state":      "UPLOADED",
					"channel_id": "cam-01",
					"object_key": "arenas/test/video.mkv",
					"snapshot": map[string]any{
						"bird_name":       "Petir",
						"registrant_name": "Joko",
					},
				}, nil
			}
			return nil, nil
		},
	}

	obj := &mockObjects{downloadURL: "https://storage.test/presigned-video.mp4"}

	uc := &usecase.UseCase{
		DB:      db,
		ACL:     &mockACL{},
		Objects: obj,
	}

	watch, err := uc.PublicWatch(ctx, assetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if watch.BirdName != "Petir" {
		t.Errorf("expected bird Petir, got %s", watch.BirdName)
	}
	if watch.OwnerName != "Joko" {
		t.Errorf("expected owner Joko, got %s", watch.OwnerName)
	}
	if watch.StreamingURL != "https://storage.test/presigned-video.mp4" {
		t.Errorf("unexpected streaming url: %s", watch.StreamingURL)
	}
}
