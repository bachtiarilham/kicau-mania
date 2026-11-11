// Request schemas describe the HTTP JSON contract without moving semantic validation out of UseCases.
package iam

// ProfilePatch describes the documented JSON body; business validation stays in the UseCase.
type ProfilePatch struct {
	Name  string `json:"name" example:"Nama Baru"`
	Phone string `json:"phone" example:"+6281234567890"`
}

// ActivePatch describes the documented JSON body; business validation stays in the UseCase.
type ActivePatch struct {
	IsActive *bool `json:"is_active" binding:"required" example:"False"`
}

// RoleGrant describes the documented JSON body; business validation stays in the UseCase.
type RoleGrant struct {
	Role string `json:"role" binding:"required" example:"peserta" enums:"superadmin,peserta,penjoki"`
}

// ArenaAssignment describes the documented JSON body; business validation stays in the UseCase.
type ArenaAssignment struct {
	UserID string `json:"user_id" binding:"required" example:"6d6f636b-0002-4000-8000-000000000001" format:"uuid"`
	Role   string `json:"role" binding:"required" example:"juri" enums:"admin_arena,operator,juri"`
}
