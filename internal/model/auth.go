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
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
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
	Enabled bool `json:"enabled"`
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
