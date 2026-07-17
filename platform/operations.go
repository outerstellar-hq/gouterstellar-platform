package platform

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo"
)

// OperationAction names independently auditable and authorizable workflows.
type OperationAction string

const (
	ActionBackupCreate   OperationAction = "backup.create"
	ActionRestorePlan    OperationAction = "restore.plan"
	ActionRestoreExecute OperationAction = "restore.execute"
)

// OperationState is the honest lifecycle reported by a remote or local
// service-owned operation.
type OperationState string

const (
	OperationQueued    OperationState = "queued"
	OperationRunning   OperationState = "running"
	OperationSucceeded OperationState = "succeeded"
	OperationFailed    OperationState = "failed"
)

type VerificationState string

const (
	VerificationPending  VerificationState = "pending"
	VerificationVerified VerificationState = "verified"
	VerificationFailed   VerificationState = "failed"
)

// BackupSnapshot is safe operational metadata. Providers must not include
// database credentials, certificate authority keys, or encryption keys.
type BackupSnapshot struct {
	ID           string
	CreatedAt    time.Time
	SizeBytes    int64
	Verification VerificationState
}

// BackupRequest carries safe provenance for a service-owned backup manifest.
// The provider remains responsible for the data, encryption, and credentials.
type BackupRequest struct {
	RequestedAt time.Time
	Build       buildinfo.Info
}

// OperationJob identifies a long-running request accepted by its owning
// service.
type OperationJob struct {
	ID      string
	State   OperationState
	Message string
}

// RestorePlan describes a validated, non-mutating restore proposal. The
// platform binds it to the requesting administrator before execution.
type RestorePlan struct {
	ID         string
	SnapshotID string
	Summary    string
	Warnings   []string
	ExpiresAt  time.Time
}

// ServiceOperationsState reports operational safety and progress without
// exposing implementation secrets.
type ServiceOperationsState struct {
	Maintenance bool
	ReadOnly    bool
	Progress    string
	ActiveJob   *OperationJob
}

// OperationsProvider is implemented by the service that owns the data. A
// provider may be an in-process adapter or a client for a separate internal
// service; the platform never receives shell commands or raw credentials.
type OperationsProvider interface {
	State(context.Context) (ServiceOperationsState, error)
	ListBackups(context.Context) ([]BackupSnapshot, error)
	RequestBackup(context.Context, BackupRequest) (OperationJob, error)
	PlanRestore(context.Context, string) (RestorePlan, error)
	ExecuteRestore(context.Context, string) (OperationJob, error)
}

type OperationsDescriptor struct {
	Label string
}

type OperationAudit struct {
	UserID    string
	Username  string
	Extension string
	Action    OperationAction
	Outcome   string
}

// OperationsPage is the shared shell view rendered for every provider.
type OperationsPage struct {
	Owner               string
	Label               string
	CSRFToken           string
	State               ServiceOperationsState
	Snapshots           []BackupSnapshot
	Job                 *OperationJob
	Plan                *RestorePlan
	RestoreConfirmation string
}

type pendingRestore struct {
	userID    string
	expiresAt time.Time
}

// OperationsRegistry stamps every provider with its extension owner and
// contributes only fixed admin-group routes for that owner.
type OperationsRegistry struct {
	owner   string
	routes  *RouteRegistry
	pages   PageRenderer
	auditor OperationsAuditor
	mu      sync.Mutex
	pending map[string]pendingRestore
}

var operationsOwnerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func newOperationsRegistry(owner string, routes *RouteRegistry, pages PageRenderer, auditor OperationsAuditor) *OperationsRegistry {
	return &OperationsRegistry{owner: owner, routes: routes, pages: pages, auditor: auditor, pending: make(map[string]pendingRestore)}
}

// Register contributes the shared operations UI and typed mutation endpoints.
func (r *OperationsRegistry) Register(descriptor OperationsDescriptor, provider OperationsProvider) error {
	if r == nil || r.routes == nil {
		return fmt.Errorf("operations registry is not configured")
	}
	if !operationsOwnerPattern.MatchString(r.owner) {
		return fmt.Errorf("extension %q cannot register operations: owner must use lowercase letters, digits, and hyphens", r.owner)
	}
	if strings.TrimSpace(descriptor.Label) == "" {
		return fmt.Errorf("extension %s operations label is required", r.owner)
	}
	if provider == nil {
		return fmt.Errorf("extension %s operations provider is nil", r.owner)
	}
	if r.pages == nil {
		return fmt.Errorf("extension %s operations require the shared page renderer", r.owner)
	}
	if r.auditor == nil {
		return fmt.Errorf("extension %s operations require an audit sink", r.owner)
	}

	base := "/admin/operations/" + r.owner
	r.routes.Admin(http.MethodGet, base, "Extension operations", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.overview(w, req, descriptor, provider, nil, nil)
	}))
	r.routes.Admin(http.MethodPost, base+"/backups", "Request extension backup", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.requestBackup(w, req, descriptor, provider)
	}))
	r.routes.Admin(http.MethodPost, base+"/restore-plans", "Plan extension restore", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.planRestore(w, req, descriptor, provider)
	}))
	r.routes.Admin(http.MethodPost, base+"/restores", "Execute confirmed extension restore", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.executeRestore(w, req, descriptor, provider)
	}))
	return nil
}

func (r *OperationsRegistry) overview(w http.ResponseWriter, req *http.Request, descriptor OperationsDescriptor, provider OperationsProvider, job *OperationJob, plan *RestorePlan) {
	requestContext := RequestContextFrom(req)
	if requestContext.User == nil || !requestContext.User.IsAdmin {
		http.Error(w, "Administrator access required", http.StatusForbidden)
		return
	}
	state, err := provider.State(req.Context())
	if err != nil {
		http.Error(w, "Operations service unavailable", http.StatusBadGateway)
		return
	}
	if err := validateJob(state.ActiveJob); err != nil {
		http.Error(w, "Operations service returned invalid state", http.StatusBadGateway)
		return
	}
	snapshots, err := provider.ListBackups(req.Context())
	if err != nil {
		http.Error(w, "Backup inventory unavailable", http.StatusBadGateway)
		return
	}
	page := OperationsPage{
		Owner: r.owner, Label: descriptor.Label, CSRFToken: requestContext.CSRFToken,
		State: state, Snapshots: snapshots, Job: job, Plan: plan,
		RestoreConfirmation: "RESTORE " + r.owner,
	}
	if err := r.pages.RenderPage(w, req, "admin_operations", page); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (r *OperationsRegistry) requestBackup(w http.ResponseWriter, req *http.Request, descriptor OperationsDescriptor, provider OperationsProvider) {
	user, ok := r.authorizeMutation(w, req, ActionBackupCreate)
	if !ok {
		return
	}
	job, err := provider.RequestBackup(req.Context(), BackupRequest{
		RequestedAt: time.Now().UTC(),
		Build:       buildinfo.Current(),
	})
	if err != nil || validateJob(&job) != nil {
		if auditErr := r.audit(req.Context(), user, ActionBackupCreate, "failed"); auditErr != nil {
			http.Error(w, "Backup failure could not be audited", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Backup request failed", http.StatusBadGateway)
		return
	}
	if err := r.audit(req.Context(), user, ActionBackupCreate, "accepted"); err != nil {
		http.Error(w, "Backup request could not be audited", http.StatusInternalServerError)
		return
	}
	r.overview(w, req, descriptor, provider, &job, nil)
}

func (r *OperationsRegistry) planRestore(w http.ResponseWriter, req *http.Request, descriptor OperationsDescriptor, provider OperationsProvider) {
	user, ok := r.authorizeMutation(w, req, ActionRestorePlan)
	if !ok {
		return
	}
	snapshotID := strings.TrimSpace(req.FormValue("snapshot_id"))
	plan, err := provider.PlanRestore(req.Context(), snapshotID)
	if err != nil || validatePlan(plan, snapshotID) != nil {
		if auditErr := r.audit(req.Context(), user, ActionRestorePlan, "failed"); auditErr != nil {
			http.Error(w, "Restore-plan failure could not be audited", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Restore plan is invalid", http.StatusBadGateway)
		return
	}
	r.mu.Lock()
	r.pending[plan.ID] = pendingRestore{userID: user.ID, expiresAt: plan.ExpiresAt}
	r.mu.Unlock()
	if err := r.audit(req.Context(), user, ActionRestorePlan, "validated"); err != nil {
		r.mu.Lock()
		delete(r.pending, plan.ID)
		r.mu.Unlock()
		http.Error(w, "Restore plan could not be audited", http.StatusInternalServerError)
		return
	}
	r.overview(w, req, descriptor, provider, nil, &plan)
}

func (r *OperationsRegistry) executeRestore(w http.ResponseWriter, req *http.Request, descriptor OperationsDescriptor, provider OperationsProvider) {
	user, ok := r.authorizeMutation(w, req, ActionRestoreExecute)
	if !ok {
		return
	}
	planID := strings.TrimSpace(req.FormValue("plan_id"))
	confirmation := strings.TrimSpace(req.FormValue("confirmation"))
	wantConfirmation := "RESTORE " + r.owner

	r.mu.Lock()
	pending, exists := r.pending[planID]
	validPlan := exists && pending.userID == user.ID && time.Now().Before(pending.expiresAt)
	if validPlan && subtle.ConstantTimeCompare([]byte(confirmation), []byte(wantConfirmation)) == 1 {
		delete(r.pending, planID)
	} else {
		validPlan = false
	}
	r.mu.Unlock()
	if !validPlan {
		if auditErr := r.audit(req.Context(), user, ActionRestoreExecute, "rejected"); auditErr != nil {
			http.Error(w, "Restore rejection could not be audited", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Restore requires a current plan and exact confirmation", http.StatusConflict)
		return
	}

	job, err := provider.ExecuteRestore(req.Context(), planID)
	if err != nil || validateJob(&job) != nil {
		if auditErr := r.audit(req.Context(), user, ActionRestoreExecute, "failed"); auditErr != nil {
			http.Error(w, "Restore failure could not be audited", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Restore request failed", http.StatusBadGateway)
		return
	}
	if err := r.audit(req.Context(), user, ActionRestoreExecute, "accepted"); err != nil {
		http.Error(w, "Restore request could not be audited", http.StatusInternalServerError)
		return
	}
	r.overview(w, req, descriptor, provider, &job, nil)
}

func (r *OperationsRegistry) authorizeMutation(w http.ResponseWriter, req *http.Request, action OperationAction) (*RequestUser, bool) {
	requestContext := RequestContextFrom(req)
	if requestContext.User == nil || !requestContext.User.IsAdmin {
		http.Error(w, "Administrator access required", http.StatusForbidden)
		return nil, false
	}
	submitted := req.FormValue("csrf_token")
	if requestContext.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(submitted), []byte(requestContext.CSRFToken)) != 1 {
		if err := r.audit(req.Context(), requestContext.User, action, "csrf_rejected"); err != nil {
			http.Error(w, "Rejected request could not be audited", http.StatusInternalServerError)
			return nil, false
		}
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return nil, false
	}
	return requestContext.User, true
}

func (r *OperationsRegistry) audit(ctx context.Context, user *RequestUser, action OperationAction, outcome string) error {
	return r.auditor.RecordOperation(ctx, OperationAudit{
		UserID: user.ID, Username: user.Username, Extension: r.owner, Action: action, Outcome: outcome,
	})
}

func validatePlan(plan RestorePlan, snapshotID string) error {
	if strings.TrimSpace(snapshotID) == "" || plan.ID == "" || plan.SnapshotID != snapshotID || !plan.ExpiresAt.After(time.Now()) {
		return fmt.Errorf("invalid restore plan")
	}
	return nil
}

func validateJob(job *OperationJob) error {
	if job == nil {
		return nil
	}
	if job.ID == "" {
		return fmt.Errorf("operation job ID is required")
	}
	switch job.State {
	case OperationQueued, OperationRunning, OperationSucceeded, OperationFailed:
		return nil
	default:
		return fmt.Errorf("invalid operation state %q", job.State)
	}
}
