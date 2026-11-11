// Request schemas describe the HTTP JSON contract without moving semantic validation out of UseCases.
package asset

// BirdTransfer describes the documented JSON body; business validation stays in the UseCase.
type BirdTransfer struct {
	ToOwnerID string `json:"to_owner_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	Reason    string `json:"reason" binding:"required" example:"Transfer kepemilikan" minLength:"1" maxLength:"2000"`
}
