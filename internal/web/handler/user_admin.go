package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

const (
	adminExportPageSize   int32 = 500
	adminAuditExportLimit int   = 10000
	adminDefaultPageSize        = 20
	adminMaxPageSize            = 100
)

type UserAdminHandler struct {
	securityService *service.SecurityService
	renderer        *web.Renderer
}

func NewUserAdminHandler(secSvc *service.SecurityService, renderer *web.Renderer) *UserAdminHandler {
	return &UserAdminHandler{
		securityService: secSvc,
		renderer:        renderer,
	}
}

// ContributeRoutes registers the admin UI routes.
func (h *UserAdminHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Admin(http.MethodGet, "/admin/users", "User management", http.HandlerFunc(h.ListUsers))
	ctx.Routes.Admin(http.MethodPost, "/admin/users/{id}/toggle-enabled", "Toggle user enabled", http.HandlerFunc(h.ToggleEnabled))
	ctx.Routes.Admin(http.MethodPost, "/admin/users/{id}/toggle-role", "Toggle user role", http.HandlerFunc(h.ToggleRole))
	ctx.Routes.Admin(http.MethodPost, "/admin/users/{id}/unlock", "Unlock user", http.HandlerFunc(h.Unlock))
	ctx.Routes.Admin(http.MethodGet, "/admin/users/export", "Export users", http.HandlerFunc(h.ExportUsers))
	ctx.Routes.Admin(http.MethodGet, "/admin/users/export/json", "Export users as JSON", http.HandlerFunc(h.ExportUsersJSON))
	ctx.Routes.Admin(http.MethodGet, "/admin/audit", "Audit log", http.HandlerFunc(h.ShowAudit))
	ctx.Routes.Admin(http.MethodGet, "/admin/audit/export", "Export audit", http.HandlerFunc(h.ExportAudit))
	ctx.Routes.Admin(http.MethodGet, "/admin/audit/export/json", "Export audit as JSON", http.HandlerFunc(h.ExportAuditJSON))
	return nil
}

func (h *UserAdminHandler) ToggleEnabled(w http.ResponseWriter, r *http.Request) {
	currentUser := web.UserFromRequest(r)
	if currentUser == nil || currentUser.Role != model.RoleAdmin {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if err := h.securityService.ToggleUserEnabled(r.Context(), currentUser.ID, targetID); err != nil {
		handleServiceError(w, err)
		return
	}
	h.ListUsers(w, r)
}

func (h *UserAdminHandler) ToggleRole(w http.ResponseWriter, r *http.Request) {
	currentUser := web.UserFromRequest(r)
	if currentUser == nil || currentUser.Role != model.RoleAdmin {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if err := h.securityService.ToggleUserRole(r.Context(), currentUser.ID, targetID); err != nil {
		handleServiceError(w, err)
		return
	}
	h.ListUsers(w, r)
}

func (h *UserAdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	pageSize, offset, page := adminPagination(r)
	currentUser := web.UserFromRequest(r)

	users, err := h.securityService.ListUsersPaged(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		h.renderError(w, r, "Failed to load users", http.StatusInternalServerError)
		return
	}

	total, err := h.securityService.CountUsers(r.Context())
	if err != nil {
		h.renderError(w, r, "Failed to count users", http.StatusInternalServerError)
		return
	}
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	userItems := make([]viewmodel.UserItem, len(users))
	for i, u := range users {
		userItems[i] = viewmodel.UserItem{
			ID:                  u.ID,
			Username:            u.Username,
			Email:               u.Email,
			Role:                u.Role,
			Enabled:             u.Enabled,
			FailedLoginAttempts: u.FailedLoginAttempts,
			IsLocked:            u.LockedUntil != nil && u.LockedUntil.After(time.Now()),
			IsSelf:              currentUser != nil && u.ID == currentUser.ID.String(),
		}
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  total,
		HasPrevious: page > 1,
		HasNext:     page < totalPages,
		PageSize:    pageSize,
		Language:    web.LanguageFromRequest(r),
	}

	if err := h.renderer.RenderPage(w, r, "admin_users", viewmodel.AdminUsersPage{
		Users:      userItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *UserAdminHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	currentUser := web.UserFromRequest(r)
	if currentUser == nil || currentUser.Role != model.RoleAdmin {
		writeError(w, http.StatusForbidden, "Admin access required")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	if err := h.securityService.UnlockAccount(r.Context(), currentUser.ID, targetID); err != nil {
		handleServiceError(w, err)
		return
	}

	h.ListUsers(w, r)
}

func adminPagination(r *http.Request) (pageSize, offset, page int) {
	pageSize = getIntParam(r, "limit", 0)
	if pageSize == 0 {
		pageSize = getIntParam(r, "pageSize", adminDefaultPageSize)
	}
	pageSize = min(max(pageSize, 1), adminMaxPageSize)
	if r.URL.Query().Has("offset") {
		offset = max(getIntParam(r, "offset", 0), 0)
		page = offset/pageSize + 1
		return
	}
	page = max(getIntParam(r, "page", 1), 1)
	offset = (page - 1) * pageSize
	return
}

func (h *UserAdminHandler) ShowAudit(w http.ResponseWriter, r *http.Request) {
	pageSize, offset, page := adminPagination(r)

	entries, err := h.securityService.GetAuditLogPaged(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		h.renderError(w, r, "Failed to load audit log", http.StatusInternalServerError)
		return
	}

	total, err := h.securityService.CountAuditEntries(r.Context())
	if err != nil {
		h.renderError(w, r, "Failed to count audit entries", http.StatusInternalServerError)
		return
	}
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	auditItems := make([]viewmodel.AuditItem, len(entries))
	for i, e := range entries {
		actorName := ""
		if e.ActorUsername != nil {
			actorName = *e.ActorUsername
		}
		targetName := ""
		if e.TargetUsername != nil {
			targetName = *e.TargetUsername
		}
		detail := ""
		if e.Detail != nil {
			detail = *e.Detail
		}

		auditItems[i] = viewmodel.AuditItem{
			ID:             e.ID,
			ActorUsername:  actorName,
			TargetUsername: targetName,
			Action:         e.Action,
			Detail:         detail,
			CreatedAt:      e.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  total,
		HasPrevious: page > 1,
		HasNext:     page < totalPages,
		PageSize:    pageSize,
		Language:    web.LanguageFromRequest(r),
	}

	if err := h.renderer.RenderPage(w, r, "admin_audit", viewmodel.AdminAuditPage{
		Entries:    auditItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *UserAdminHandler) ExportUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.allUsersForExport(r.Context())
	if err != nil {
		h.renderError(w, r, "Failed to export users", http.StatusInternalServerError)
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

type userJSONExportRow struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Enabled  bool   `json:"enabled"`
}

func (h *UserAdminHandler) ExportUsersJSON(w http.ResponseWriter, r *http.Request) {
	users, err := h.allUsersForExport(r.Context())
	if err != nil {
		h.renderError(w, r, "Failed to export users", http.StatusInternalServerError)
		return
	}

	rows := make([]userJSONExportRow, len(users))
	for i, user := range users {
		rows[i] = userJSONExportRow{Username: user.Username, Email: user.Email, Role: user.Role, Enabled: user.Enabled}
	}
	if err := writeJSONDownload(w, "users.json", rows); err != nil {
		handleServiceError(w, err)
	}
}

func (h *UserAdminHandler) ExportAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := h.auditEntriesForExport(r.Context())
	if err != nil {
		h.renderError(w, r, "Failed to export audit log", http.StatusInternalServerError)
		return
	}

	headers := []string{"Timestamp", "Actor", "Action", "Target", "Detail"}
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
		rows[i] = []string{e.CreatedAt.Format(time.RFC3339), actor, e.Action, target, detail}
	}

	if err := writeCSV(w, "audit.csv", headers, rows); err != nil {
		handleServiceError(w, err)
	}
}

type auditJSONExportRow struct {
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
}

func (h *UserAdminHandler) ExportAuditJSON(w http.ResponseWriter, r *http.Request) {
	entries, err := h.auditEntriesForExport(r.Context())
	if err != nil {
		h.renderError(w, r, "Failed to export audit log", http.StatusInternalServerError)
		return
	}

	rows := make([]auditJSONExportRow, len(entries))
	for i, entry := range entries {
		actor := ""
		if entry.ActorUsername != nil {
			actor = *entry.ActorUsername
		}
		target := ""
		if entry.TargetUsername != nil {
			target = *entry.TargetUsername
		}
		detail := ""
		if entry.Detail != nil {
			detail = *entry.Detail
		}
		rows[i] = auditJSONExportRow{
			Timestamp: entry.CreatedAt, Actor: actor, Action: entry.Action, Target: target, Detail: detail,
		}
	}
	if err := writeJSONDownload(w, "audit.json", rows); err != nil {
		handleServiceError(w, err)
	}
}

func (h *UserAdminHandler) allUsersForExport(ctx context.Context) ([]model.UserSummary, error) {
	var users []model.UserSummary
	for offset := int32(0); ; offset += adminExportPageSize {
		page, err := h.securityService.ListUsersPaged(ctx, adminExportPageSize, offset)
		if err != nil {
			return nil, err
		}
		users = append(users, page...)
		if len(page) < int(adminExportPageSize) {
			return users, nil
		}
	}
}

func (h *UserAdminHandler) auditEntriesForExport(ctx context.Context) ([]model.AuditEntry, error) {
	entries := make([]model.AuditEntry, 0, adminAuditExportLimit)
	for offset := int32(0); len(entries) < adminAuditExportLimit; offset += adminExportPageSize {
		page, err := h.securityService.GetAuditLogPaged(ctx, adminExportPageSize, offset)
		if err != nil {
			return nil, err
		}
		remaining := adminAuditExportLimit - len(entries)
		if len(page) > remaining {
			page = page[:remaining]
		}
		entries = append(entries, page...)
		if len(page) < int(adminExportPageSize) {
			break
		}
	}
	return entries, nil
}

func (h *UserAdminHandler) renderError(w http.ResponseWriter, r *http.Request, message string, status int) {
	_ = h.renderer.RenderWithStatus(w, r, "error", viewmodel.ErrorPage{
		StatusCode: status,
		Title:      "Error",
		Message:    message,
	}, status)
}
