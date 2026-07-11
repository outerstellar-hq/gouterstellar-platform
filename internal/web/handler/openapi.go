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
			"/sync": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Pull message changes since timestamp",
					"parameters": []map[string]interface{}{
						{"name": "since", "in": "query", "schema": map[string]string{"type": "integer"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Sync pull response"},
					},
				},
				"post": map[string]interface{}{
					"summary": "Push message changes",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Sync push response"},
					},
				},
			},
			"/sync/contacts": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Pull contact changes since timestamp",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Contact sync pull response"},
					},
				},
				"post": map[string]interface{}{
					"summary": "Push contact changes",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Contact sync push response"},
					},
				},
			},
			"/auth/login": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Authenticate and create session",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Login successful"},
						"401": map[string]interface{}{"description": "Invalid credentials"},
					},
				},
			},
			"/auth/register": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Register a new user account",
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Account created"},
						"409": map[string]interface{}{"description": "Username taken"},
					},
				},
			},
			"/auth/profile": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "Get current user profile",
					"security": []map[string]interface{}{{"bearerAuth": map[string]interface{}{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "User profile"},
					},
				},
			},
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List all users (admin only)",
					"security": []map[string]interface{}{{"bearerAuth": map[string]interface{}{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "User list"},
						"403": map[string]interface{}{"description": "Forbidden"},
					},
				},
			},
			"/notifications": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":  "List notifications",
					"security": []map[string]interface{}{{"bearerAuth": map[string]interface{}{}}},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Notification list"},
					},
				},
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
