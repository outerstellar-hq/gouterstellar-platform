package model

import "time"

type ApiKey struct {
	ID         int64      `json:"id"`
	UserID     string     `json:"userId"`
	KeyHash    string     `json:"keyHash"`
	KeyPrefix  string     `json:"keyPrefix"`
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

type ApiKeySummary struct {
	ID         int64   `json:"id"`
	KeyPrefix  string  `json:"keyPrefix"`
	Name       string  `json:"name"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
}
