// Package model defines registration, payment and round workflow requests.
package model

import "encoding/json"

type Entry struct {
	RoundID string `json:"round_id"`
	BirdID  string `json:"bird_id"`
}
type Registration struct {
	ArenaID      string  `json:"arena_id"`
	Source       string  `json:"registration_source"`
	RegisteredBy string  `json:"registered_by_user_id"`
	Name         string  `json:"registrant_name"`
	Phone        string  `json:"registrant_phone"`
	Items        []Entry `json:"items"`
}
type Payment struct {
	Amount json.Number `json:"amount"`
	Method string      `json:"method"`
	Proof  string      `json:"proof_reference"`
}
type Allocation struct {
	ItemID      string `json:"registration_item_id"`
	GantanganID string `json:"gantangan_id"`
}
