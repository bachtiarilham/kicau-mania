// Request schemas describe the HTTP JSON contract without moving semantic validation out of UseCases.
package auth

// EmailRequest describes the documented JSON body; business validation stays in the UseCase.
type EmailRequest struct {
	Email string `json:"email" binding:"required" example:"peserta.1@mock.kicau.test" format:"email"`
}

// VerifyEmail describes the documented JSON body; business validation stays in the UseCase.
type VerifyEmail struct {
	Token string `json:"token" binding:"required"`
}

// ResetPassword describes the documented JSON body; business validation stays in the UseCase.
type ResetPassword struct {
	Token string `json:"token" binding:"required"`
	// Panjang 12–256 byte, diperiksa server.
	Password string `json:"password" binding:"required" format:"password" minLength:"12" maxLength:"256"`
}

// ChangePassword describes the documented JSON body; business validation stays in the UseCase.
type ChangePassword struct {
	OldPassword string `json:"old_password" binding:"required" format:"password"`
	// Panjang 12–256 byte, diperiksa server.
	Password string `json:"password" binding:"required" format:"password" minLength:"12" maxLength:"256"`
}

// GoogleLink describes the documented JSON body; business validation stays in the UseCase.
type GoogleLink struct {
	Credential string `json:"credential" binding:"required"`
}

// Empty describes the documented JSON body; business validation stays in the UseCase.
type Empty struct {
}
