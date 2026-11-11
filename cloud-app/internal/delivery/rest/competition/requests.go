// Request schemas describe the HTTP JSON contract without moving semantic validation out of UseCases.
package competition

import input "kicau.cloud/control/internal/competition/model"

// VerifyPayment describes the documented JSON body; business validation stays in the UseCase.
type VerifyPayment struct {
	Allocations []input.Allocation `json:"allocations" binding:"required"`
}

// JudgeAssignment describes the documented JSON body; business validation stays in the UseCase.
type JudgeAssignment struct {
	UserID string `json:"user_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
}

// CloseRound describes the documented JSON body; business validation stays in the UseCase.
type CloseRound struct {
	WinnerRegistrationItemID string `json:"winner_registration_item_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
}
