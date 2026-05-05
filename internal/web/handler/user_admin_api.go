package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type UserAdminAPI struct {
	securityService *service.SecurityService
}

func NewUserAdminAPI(secSvc *service.SecurityService) *UserAdminAPI {
	return &UserAdminAPI{securityService: secSvc}
}

func (h *UserAdminAPI) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/users", h.ListUsers)
	r.Get("/api/v1/users/count", h.CountUsers)
	r.Put("/api/v1/users/{id}/enabled", h.SetEnabled)
	r.Put("/api/v1/users/{id}/role", h.SetRole)
	r.Get("/api/v1/admin/users/export", h.ExportUsersCSV)
	r.Get("/api/v1/admin/audit/export", h.ExportAuditCSV)
}

func (h *UserAdminAPI) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	users, err := h.securityService.ListUsersPaged(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, users)
}

func (h *UserAdminAPI) CountUsers(w http.ResponseWriter, r *http.Request) {
	count, err := h.securityService.CountUsers(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"count": count})
}

func (h *UserAdminAPI) SetEnabled(w http.ResponseWriter, r *http.Request) {
	currentUser := web.UserFromRequest(r)
	if currentUser == nil || currentUser.Role != model.RoleAdmin {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req model.SetUserEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err = h.securityService.SetUserEnabled(r.Context(), currentUser.ID, targetID, req.Enabled)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "User enabled status updated"})
}

func (h *UserAdminAPI) SetRole(w http.ResponseWriter, r *http.Request) {
	currentUser := web.UserFromRequest(r)
	if currentUser == nil || currentUser.Role != model.RoleAdmin {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req model.SetUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err = h.securityService.SetUserRole(r.Context(), currentUser.ID, targetID, model.UserRole(req.Role))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "User role updated"})
}

func (h *UserAdminAPI) ExportUsersCSV(w http.ResponseWriter, r *http.Request) {
	users, err := h.securityService.ListUsersPaged(r.Context(), 10000, 0)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	headers := []string{"Username", "Email", "Role", "Enabled"}
	rows := make([][]string, len(users))
	for i, u := range users {
		rows[i] = []string{u.Username, u.Email, string(u.Role), strconv.FormatBool(u.Enabled)}
	}

	writeCSV(w, "users.csv", headers, rows)
}

func (h *UserAdminAPI) ExportAuditCSV(w http.ResponseWriter, r *http.Request) {
	entries, err := h.securityService.GetAuditLogPaged(r.Context(), 10000, 0)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	headers := []string{"Actor", "Target", "Action", "Detail", "Created At"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		actor := ""
		if e.ActorUsername != nil {
			actor = *e.ActorUsername
		}
		target := ""
		if e.TargetUsername != nil {
			target = *e.TargetUsername
		}
		detail := ""
		if e.Detail != nil {
			detail = *e.Detail
		}
		rows[i] = []string{actor, target, e.Action, detail, e.CreatedAt.Format("2006-01-02 15:04:05")}
	}

	writeCSV(w, "audit_log.csv", headers, rows)
}
