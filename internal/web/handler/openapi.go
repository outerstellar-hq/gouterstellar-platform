package handler

import (
	"encoding/json"
	"net/http"
)

// OpenAPIHandler serves a hand-written OpenAPI 3.0 spec describing the
// platform's key API endpoints. The spec is served at /openapi.json and gives
// API consumers a contract document without requiring a full Huma migration of
// the existing Chi handlers.
type OpenAPIHandler struct{}

// NewOpenAPIHandler constructs an OpenAPIHandler.
func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{}
}

// Spec writes the OpenAPI document as JSON.
func (h *OpenAPIHandler) Spec(w http.ResponseWriter, r *http.Request) {
	spec := buildOpenAPISpec()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(spec)
}

// buildOpenAPISpec returns the static OpenAPI 3.0 document for the platform
// API. Paths are relative to the /api/v1 server URL declared below.
func buildOpenAPISpec() map[string]interface{} {
	bearer := []map[string]interface{}{{"bearerAuth": map[string]interface{}{}}}
	ok := func(desc string) map[string]interface{} {
		return map[string]interface{}{"200": map[string]interface{}{"description": desc}}
	}
	op := func(summary string, secured bool, responses map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{
			"summary":   summary,
			"responses": responses,
		}
		if secured {
			m["security"] = bearer
		}
		return m
	}

	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Outerstellar Platform API",
			"version":     "1.0.0",
			"description": "Sync, auth, admin, and notification APIs for the Outerstellar Platform.",
		},
		"servers": []map[string]interface{}{
			{"url": "/api/v1", "description": "Current server"},
		},
		"paths": map[string]interface{}{
			// Sync
			"/sync": map[string]interface{}{
				"get":  op("Pull message changes since timestamp", true, ok("Sync pull response")),
				"post": op("Push message changes", true, ok("Sync push response")),
			},
			"/sync/contacts": map[string]interface{}{
				"get":  op("Pull contact changes since timestamp", true, ok("Contact sync pull response")),
				"post": op("Push contact changes", true, ok("Contact sync push response")),
			},
			// Auth
			"/auth/login": map[string]interface{}{
				"post": op("Authenticate and create session", false, map[string]interface{}{
					"200": map[string]interface{}{"description": "Login successful"},
					"401": map[string]interface{}{"description": "Invalid credentials"},
				}),
			},
			"/auth/totp/verify": map[string]interface{}{
				"post": op("Verify a TOTP or backup code and create a session", false, map[string]interface{}{
					"200": map[string]interface{}{"description": "TOTP verified"},
					"401": map[string]interface{}{"description": "Invalid, expired, or locked challenge"},
				}),
			},
			"/auth/totp/setup": map[string]interface{}{
				"post": op("Create an authenticator enrollment secret", true, ok("TOTP setup")),
			},
			"/auth/totp/confirm": map[string]interface{}{
				"post": op("Verify and enable authenticator enrollment", true, map[string]interface{}{
					"201": map[string]interface{}{"description": "TOTP enabled with one-time backup codes"},
					"400": map[string]interface{}{"description": "Invalid code"},
				}),
			},
			"/auth/totp/disable": map[string]interface{}{
				"post": op("Disable TOTP after password confirmation", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "TOTP disabled"},
					"401": map[string]interface{}{"description": "Invalid password"},
				}),
			},
			"/auth/register": map[string]interface{}{
				"post": op("Register a new user account", false, map[string]interface{}{
					"201": map[string]interface{}{"description": "Account created"},
					"400": map[string]interface{}{"description": "Invalid username or password"},
					"403": map[string]interface{}{"description": "Registration disabled"},
					"409": map[string]interface{}{"description": "Username taken"},
				}),
			},
			"/auth/token": map[string]interface{}{
				"post": op("Issue a token (session/API key/JWT)", false, map[string]interface{}{
					"200": map[string]interface{}{"description": "Token issued"},
					"401": map[string]interface{}{"description": "Invalid credentials"},
				}),
			},
			"/auth/logout": map[string]interface{}{
				"post": op("Invalidate the current session", true, ok("Session invalidated")),
			},
			"/auth/change-password": map[string]interface{}{
				"post": op("Change the current user's password", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "Password changed"},
					"400": map[string]interface{}{"description": "Invalid input"},
				}),
			},
			"/auth/reset-password": map[string]interface{}{
				"post": op("Request a password reset email", false, ok("Reset requested")),
			},
			"/auth/confirm-reset": map[string]interface{}{
				"post": op("Confirm a password reset with a token", false, map[string]interface{}{
					"200": map[string]interface{}{"description": "Password reset"},
					"400": map[string]interface{}{"description": "Invalid or expired token"},
				}),
			},
			"/auth/profile": map[string]interface{}{
				"get": op("Get current user profile", true, ok("User profile")),
				"put": op("Update current user profile", true, ok("Profile updated")),
			},
			"/auth/api-keys": map[string]interface{}{
				"post": op("Create a new API key", true, map[string]interface{}{
					"201": map[string]interface{}{"description": "API key created"},
				}),
				"get": op("List API keys for the current user", true, ok("API key list")),
			},
			"/auth/api-keys/{id}": map[string]interface{}{
				"delete": op("Delete an API key by id", true, map[string]interface{}{
					"204": map[string]interface{}{"description": "API key deleted"},
					"404": map[string]interface{}{"description": "API key not found"},
				}),
			},
			// Users / admin
			"/users": map[string]interface{}{
				"get": op("List all users (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "User list"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/users/count": map[string]interface{}{
				"get": op("Count all users (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "User count"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/users/{id}/enabled": map[string]interface{}{
				"put": op("Enable or disable a user (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "User enabled state updated"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/users/{id}/role": map[string]interface{}{
				"put": op("Change a user's role (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "User role updated"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/admin/users/export": map[string]interface{}{
				"get": op("Export users as CSV (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "CSV download"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/admin/audit/export": map[string]interface{}{
				"get": op("Export audit log as CSV (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "CSV download"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/admin/export/message/csv": map[string]interface{}{
				"get": op("Export messages as CSV (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "CSV download"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/admin/export/message/json": map[string]interface{}{
				"get": op("Export messages as JSON (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "JSON download"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/admin/export/contact/csv": map[string]interface{}{
				"get": op("Export contacts as CSV (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "CSV download"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			"/admin/export/contact/json": map[string]interface{}{
				"get": op("Export contacts as JSON (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "JSON download"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
			// Notifications
			"/notifications": map[string]interface{}{
				"get": op("List notifications", true, ok("Notification list")),
				"delete": op("Delete notifications", true, map[string]interface{}{
					"204": map[string]interface{}{"description": "Deleted"},
				}),
			},
			"/notifications/{id}/read": map[string]interface{}{
				"put": op("Mark a single notification as read", true, ok("Notification marked read")),
			},
			"/notifications/read-all": map[string]interface{}{
				"put": op("Mark all notifications as read", true, ok("All notifications marked read")),
			},
			"/notifications/{id}": map[string]interface{}{
				"delete": op("Delete a single notification", true, map[string]interface{}{
					"204": map[string]interface{}{"description": "Notification deleted"},
					"404": map[string]interface{}{"description": "Notification not found"},
				}),
			},
			// Devices
			"/devices/register": map[string]interface{}{
				"post": op("Register a device push token", true, map[string]interface{}{
					"201": map[string]interface{}{"description": "Device registered"},
					"400": map[string]interface{}{"description": "Invalid input"},
				}),
			},
			"/devices/{id}": map[string]interface{}{
				"delete": op("Unregister a device token", true, map[string]interface{}{
					"204": map[string]interface{}{"description": "Device removed"},
					"404": map[string]interface{}{"description": "Device not found"},
				}),
			},
			// Reports
			"/reports/summary": map[string]interface{}{
				"get": op("Aggregate report summary (admin only)", true, map[string]interface{}{
					"200": map[string]interface{}{"description": "Report summary"},
					"403": map[string]interface{}{"description": "Forbidden"},
				}),
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":   "http",
					"scheme": "bearer",
				},
			},
		},
	}
}
