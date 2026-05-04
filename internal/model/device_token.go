package model

type DeviceToken struct {
	ID        int64
	UserID    string
	Platform  string
	Token     string
	AppBundle *string
}
