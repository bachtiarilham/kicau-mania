// These request schemas document the generic map-based CRUD handler. They do not run validation.
package crud

// ArenaCreate describes the documented JSON body; business validation stays in the UseCase.
type ArenaCreate struct {
	Name     string `json:"name" binding:"required" example:"Arena Contoh"`
	Address  string `json:"address" example:"Jakarta"`
	Timezone string `json:"timezone" example:"Asia/Jakarta"`
	IsActive bool   `json:"is_active" example:"True"`
}

// ArenaPatch describes the documented JSON body; business validation stays in the UseCase.
type ArenaPatch struct {
	Name     string `json:"name" example:"Arena Contoh"`
	Address  string `json:"address" example:"Jakarta"`
	Timezone string `json:"timezone" example:"Asia/Jakarta"`
	IsActive bool   `json:"is_active" example:"True"`
}

// OwnerCreate describes the documented JSON body; business validation stays in the UseCase.
type OwnerCreate struct {
	Name   string `json:"name" binding:"required" example:"Pemilik Contoh"`
	Phone  string `json:"phone" example:"+6281234567890"`
	UserID string `json:"user_id" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
}

// OwnerPatch describes the documented JSON body; business validation stays in the UseCase.
type OwnerPatch struct {
	Name   string `json:"name" example:"Pemilik Contoh"`
	Phone  string `json:"phone" example:"+6281234567890"`
	UserID string `json:"user_id" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
}

// BirdCreate describes the documented JSON body; business validation stays in the UseCase.
type BirdCreate struct {
	OwnerID    string `json:"owner_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	Name       string `json:"name" binding:"required" example:"Burung Contoh"`
	BirdType   string `json:"bird_type" binding:"required" example:"MURAI"`
	RingNumber string `json:"ring_number" example:"MOCK-RING-001"`
}

// BirdPatch describes the documented JSON body; business validation stays in the UseCase.
type BirdPatch struct {
	Name       string `json:"name" example:"Burung Contoh"`
	BirdType   string `json:"bird_type" example:"MURAI"`
	RingNumber string `json:"ring_number" example:"MOCK-RING-001"`
}

// GantanganCreate describes the documented JSON body; business validation stays in the UseCase.
type GantanganCreate struct {
	Number      int    `json:"number" binding:"required" example:"1" minimum:"1"`
	Description string `json:"description" example:"Gantangan 1"`
}

// GantanganPatch describes the documented JSON body; business validation stays in the UseCase.
type GantanganPatch struct {
	Number      int    `json:"number" example:"1" minimum:"1"`
	Description string `json:"description" example:"Gantangan 1"`
}

// EngineCreate describes the documented JSON body; business validation stays in the UseCase.
type EngineCreate struct {
	Name     string `json:"name" binding:"required" example:"Engine Arena"`
	IsActive bool   `json:"is_active" example:"True"`
}

// EnginePatch describes the documented JSON body; business validation stays in the UseCase.
type EnginePatch struct {
	Name     string `json:"name" example:"Engine Arena"`
	IsActive bool   `json:"is_active" example:"True"`
}

// DeviceCreate describes the documented JSON body; business validation stays in the UseCase.
type DeviceCreate struct {
	EngineID    string `json:"engine_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	GantanganID string `json:"gantangan_id" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	Type        string `json:"type" binding:"required" example:"CAMERA"`
	IPAddress   string `json:"ip_address" binding:"required" example:"192.168.1.10"`
	// URL rtsp/rtsps. Tidak dikembalikan pada GET.
	RtspURL string `json:"rtsp_url" binding:"required"`
}

// DevicePatch describes the documented JSON body; business validation stays in the UseCase.
type DevicePatch struct {
	EngineID    string `json:"engine_id" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	GantanganID string `json:"gantangan_id" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	Type        string `json:"type" example:"CAMERA"`
	IPAddress   string `json:"ip_address" example:"192.168.1.10"`
	// URL rtsp/rtsps. Tidak dikembalikan pada GET.
	RtspURL string `json:"rtsp_url"`
}

// EventCreate describes the documented JSON body; business validation stays in the UseCase.
type EventCreate struct {
	Title    string `json:"title" binding:"required" example:"Event Contoh"`
	Date     string `json:"date" binding:"required" example:"2026-11-14" format:"date"`
	Location string `json:"location" binding:"required" example:"Arena Contoh"`
	// Opsional; mengikuti status yang didukung skema database.
	Status string `json:"status"`
}

// EventPatch describes the documented JSON body; business validation stays in the UseCase.
type EventPatch struct {
	Title    string `json:"title" example:"Event Contoh"`
	Date     string `json:"date" example:"2026-11-14" format:"date"`
	Location string `json:"location" example:"Arena Contoh"`
	// Opsional; mengikuti status yang didukung skema database.
	Status string `json:"status"`
}

// CategoryCreate describes the documented JSON body; business validation stays in the UseCase.
type CategoryCreate struct {
	Code string `json:"code" binding:"required" example:"UMUM"`
	Name string `json:"name" binding:"required" example:"Kategori Umum"`
}

// CategoryPatch describes the documented JSON body; business validation stays in the UseCase.
type CategoryPatch struct {
	Code string `json:"code" example:"UMUM"`
	Name string `json:"name" example:"Kategori Umum"`
}

// RoundCreate describes the documented JSON body; business validation stays in the UseCase.
type RoundCreate struct {
	EventID                string  `json:"event_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	CategoryID             string  `json:"category_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	RoundNumber            int     `json:"round_number" binding:"required" example:"1" minimum:"1"`
	BirdType               string  `json:"bird_type" binding:"required" example:"MURAI"`
	EntryFee               float64 `json:"entry_fee" binding:"required" example:"50000" minimum:"0"`
	SessionDurationMinutes int     `json:"session_duration_minutes" example:"10" minimum:"1"`
	JudgingDurationMinutes int     `json:"judging_duration_minutes" example:"5" minimum:"1"`
}

// RoundPatch describes the documented JSON body; business validation stays in the UseCase.
type RoundPatch struct {
	EventID                string  `json:"event_id" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	CategoryID             string  `json:"category_id" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	RoundNumber            int     `json:"round_number" example:"1" minimum:"1"`
	BirdType               string  `json:"bird_type" example:"MURAI"`
	EntryFee               float64 `json:"entry_fee" example:"50000" minimum:"0"`
	SessionDurationMinutes int     `json:"session_duration_minutes" example:"10" minimum:"1"`
	JudgingDurationMinutes int     `json:"judging_duration_minutes" example:"5" minimum:"1"`
}

// CriterionCreate describes the documented JSON body; business validation stays in the UseCase.
type CriterionCreate struct {
	CriteriaName string `json:"criteria_name" binding:"required" example:"Volume" minLength:"1" maxLength:"100"`
}

// CriterionPatch describes the documented JSON body; business validation stays in the UseCase.
type CriterionPatch struct {
	CriteriaName string `json:"criteria_name" example:"Volume" minLength:"1" maxLength:"100"`
}
