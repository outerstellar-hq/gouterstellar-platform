package model

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type AuthTokenResponse struct {
	Status       string `json:"status,omitempty"`
	Token        string `json:"token,omitempty"`
	PartialToken string `json:"partialToken,omitempty"`
	Username     string `json:"username,omitempty"`
	Role         string `json:"role,omitempty"`
}

type AuthenticationResult interface {
	isAuthenticationResult()
}

type Authenticated struct {
	User *User
}

func (Authenticated) isAuthenticationResult() {}

type TOTPRequired struct {
	PartialToken string
}

func (TOTPRequired) isAuthenticationResult() {}

type TOTPVerifyRequest struct {
	PartialToken string `json:"partialToken"`
	Code         string `json:"code"`
}

type TOTPVerifyResponse struct {
	Status   string `json:"status"`
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Role     string `json:"role,omitempty"`
}

type TOTPSetupResponse struct {
	Secret    string `json:"secret"`
	QRDataURI string `json:"qrDataUri"`
}

type TOTPConfirmRequest struct {
	Secret string `json:"secret"`
	Code   string `json:"code"`
}

type TOTPConfirmResponse struct {
	Status      string   `json:"status"`
	BackupCodes []string `json:"backupCodes,omitempty"`
}

type TOTPDisableRequest struct {
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required"`
}

type UpdateProfileRequest struct {
	Email     string  `json:"email" validate:"required,email"`
	Username  *string `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
}

type UpdateNotificationPrefsRequest struct {
	EmailEnabled bool `json:"emailEnabled"`
	PushEnabled  bool `json:"pushEnabled"`
}

type DeleteAccountRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
}

type UserProfileResponse struct {
	Username                  string  `json:"username"`
	Email                     string  `json:"email"`
	AvatarURL                 *string `json:"avatarUrl"`
	EmailNotificationsEnabled bool    `json:"emailNotificationsEnabled"`
	PushNotificationsEnabled  bool    `json:"pushNotificationsEnabled"`
}

type SetUserEnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

type SetUserRoleRequest struct {
	Role string `json:"role" validate:"required"`
}

type CreateApiKeyRequest struct {
	Name string `json:"name" validate:"required"`
}

type CreateApiKeyResponse struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	KeyPrefix string `json:"keyPrefix"`
}

type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type PasswordResetConfirm struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"newPassword" validate:"required"`
}
