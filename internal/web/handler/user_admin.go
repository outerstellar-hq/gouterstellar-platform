package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
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

func (h *UserAdminHandler) RegisterRoutes(r chi.Router) {
	r.Get("/admin/users", h.ListUsers)
	r.Post("/admin/users/{id}/enabled", h.SetEnabled)
	r.Post("/admin/users/{id}/role", h.SetRole)
	r.Get("/admin/users/export", h.ExportUsers)
	r.Get("/admin/audit", h.ShowAudit)
	r.Get("/admin/audit/export", h.ExportAudit)
}

func (h *UserAdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 20)
	offset := (page - 1) * pageSize

	users, err := h.securityService.ListUsersPaged(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		h.renderError(w, r, "Failed to load users", http.StatusInternalServerError)
		return
	}

	total, _ := h.securityService.CountUsers(r.Context())
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	userItems := make([]viewmodel.UserItem, len(users))
	for i, u := range users {
		userItems[i] = viewmodel.UserItem{
			ID:       u.ID,
			Username: u.Username,
			Email:    u.Email,
			Role:     u.Role,
			Enabled:  u.Enabled,
		}
	}

	pagination := viewmodel.PaginationInfo{
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  total,
		HasPrevious: page > 1,
		HasNext:     page < totalPages,
		PageSize:    pageSize,
	}

	if err := h.renderer.Render(w, r, "admin_users.html", viewmodel.AdminUsersPage{
		Users:      userItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *UserAdminHandler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	currentUser := web.UserFromRequest(r)

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	enabled := r.FormValue("enabled") == "true"

	err = h.securityService.SetUserEnabled(r.Context(), currentUser.ID, targetID, enabled)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *UserAdminHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	currentUser := web.UserFromRequest(r)

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form submission")
		return
	}

	role := r.FormValue("role")

	err = h.securityService.SetUserRole(r.Context(), currentUser.ID, targetID, model.UserRole(role))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *UserAdminHandler) ShowAudit(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r, "page", 1)
	pageSize := getIntParam(r, "pageSize", 50)
	offset := (page - 1) * pageSize

	entries, err := h.securityService.GetAuditLogPaged(r.Context(), safeInt32(pageSize), safeInt32(offset))
	if err != nil {
		h.renderError(w, r, "Failed to load audit log", http.StatusInternalServerError)
		return
	}

	total, _ := h.securityService.CountAuditEntries(r.Context())
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
	}

	if err := h.renderer.Render(w, r, "admin_audit.html", viewmodel.AdminAuditPage{
		Entries:    auditItems,
		Pagination: pagination,
	}); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *UserAdminHandler) ExportUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.securityService.ListUsersPaged(r.Context(), 10000, 0)
	if err != nil {
		h.renderError(w, r, "Failed to export users", http.StatusInternalServerError)
		return
	}

	headers := []string{"Username", "Email", "Role", "Enabled"}
	rows := make([][]string, len(users))
	for i, u := range users {
		rows[i] = []string{u.Username, u.Email, string(u.Role), strconv.FormatBool(u.Enabled)}
	}

	writeCSV(w, "users.csv", headers, rows)
}

func (h *UserAdminHandler) ExportAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := h.securityService.GetAuditLogPaged(r.Context(), 10000, 0)
	if err != nil {
		h.renderError(w, r, "Failed to export audit log", http.StatusInternalServerError)
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

func (h *UserAdminHandler) renderError(w http.ResponseWriter, r *http.Request, message string, status int) {
	_ = h.renderer.RenderWithStatus(w, r, "error.html", viewmodel.ErrorPage{
		StatusCode: status,
		Title:      "Error",
		Message:    message,
	}, status)
}
