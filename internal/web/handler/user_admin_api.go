package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type UserAdminAPI struct {
	securityService *service.SecurityService
}

func NewUserAdminAPI(secSvc *service.SecurityService) *UserAdminAPI {
	return &UserAdminAPI{securityService: secSvc}
}

// ContributeRoutes registers the user admin API routes (bearer auth applied by builder).
func (h *UserAdminAPI) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/users", "List users", requireAdminAPI(http.HandlerFunc(h.ListUsers)))
	ctx.Routes.API(http.MethodPut, "/api/v1/admin/users/{id}/enabled", "Set user enabled", requireAdminAPI(http.HandlerFunc(h.SetEnabled)))
	ctx.Routes.API(http.MethodPut, "/api/v1/admin/users/{id}/role", "Set user role", requireAdminAPI(http.HandlerFunc(h.SetRole)))
	ctx.Routes.API(http.MethodGet, "/api/v1/users", "List users", requireAdminAPI(http.HandlerFunc(h.ListUsers)))
	ctx.Routes.API(http.MethodGet, "/api/v1/users/count", "Count users", requireAdminAPI(http.HandlerFunc(h.CountUsers)))
	ctx.Routes.API(http.MethodPut, "/api/v1/users/{id}/enabled", "Set user enabled", requireAdminAPI(http.HandlerFunc(h.SetEnabled)))
	ctx.Routes.API(http.MethodPut, "/api/v1/users/{id}/role", "Set user role", requireAdminAPI(http.HandlerFunc(h.SetRole)))
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/users/export", "Export users CSV", requireAdminAPI(http.HandlerFunc(h.ExportUsersCSV)))
	ctx.Routes.API(http.MethodGet, "/api/v1/admin/audit/export", "Export audit CSV", requireAdminAPI(http.HandlerFunc(h.ExportAuditCSV)))
	return nil
}

func (h *UserAdminAPI) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.securityService.ListUsers(r.Context())
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
	if req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "Enabled is required")
		return
	}

	err = h.securityService.SetUserEnabled(r.Context(), currentUser.ID, targetID, *req.Enabled)
	if err != nil {
		handleAdminMutationError(w, err)
		return
	}

	writeText(w, http.StatusOK, "User updated")
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
	role := model.UserRole(strings.ToUpper(req.Role))
	if role != model.RoleUser && role != model.RoleAdmin {
		writeText(w, http.StatusBadRequest, "Invalid role")
		return
	}

	err = h.securityService.SetUserRole(r.Context(), currentUser.ID, targetID, role)
	if err != nil {
		handleAdminMutationError(w, err)
		return
	}

	writeText(w, http.StatusOK, "User role updated")
}

func handleAdminMutationError(w http.ResponseWriter, err error) {
	var notFound *model.UserNotFoundError
	var notAllowed *model.InsufficientPermissionError
	switch {
	case errors.As(err, &notFound):
		writeText(w, http.StatusNotFound, err.Error())
	case errors.As(err, &notAllowed):
		writeText(w, http.StatusBadRequest, err.Error())
	default:
		handleServiceError(w, err)
	}
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

	if err := writeCSV(w, "users.csv", headers, rows); err != nil {
		handleServiceError(w, err)
	}
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

	if err := writeCSV(w, "audit_log.csv", headers, rows); err != nil {
		handleServiceError(w, err)
	}
}
