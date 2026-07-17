package platform

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type operationsTestRenderer struct {
	page string
	data OperationsPage
}

func (*operationsTestRenderer) RegisterTemplates(string, fs.FS, string, string) error { return nil }
func (r *operationsTestRenderer) RenderPage(_ http.ResponseWriter, _ *http.Request, page string, data any) error {
	r.page = page
	r.data = data.(OperationsPage)
	return nil
}

type operationsTestAuditor struct {
	events []OperationAudit
}

func (a *operationsTestAuditor) RecordOperation(_ context.Context, event OperationAudit) error {
	a.events = append(a.events, event)
	return nil
}

type operationsTestProvider struct {
	state         ServiceOperationsState
	snapshots     []BackupSnapshot
	backupJob     OperationJob
	restorePlan   RestorePlan
	restoreJob    OperationJob
	executedPlan  string
	backupRequest BackupRequest
}

func (p *operationsTestProvider) State(context.Context) (ServiceOperationsState, error) {
	return p.state, nil
}
func (p *operationsTestProvider) ListBackups(context.Context) ([]BackupSnapshot, error) {
	return p.snapshots, nil
}
func (p *operationsTestProvider) RequestBackup(_ context.Context, request BackupRequest) (OperationJob, error) {
	p.backupRequest = request
	return p.backupJob, nil
}
func (p *operationsTestProvider) PlanRestore(_ context.Context, snapshotID string) (RestorePlan, error) {
	plan := p.restorePlan
	plan.SnapshotID = snapshotID
	return plan, nil
}
func (p *operationsTestProvider) ExecuteRestore(_ context.Context, planID string) (OperationJob, error) {
	p.executedPlan = planID
	return p.restoreJob, nil
}

func operationsTestContext(owner string) (*ContributionContext, *operationsTestRenderer, *operationsTestAuditor) {
	renderer := &operationsTestRenderer{}
	auditor := &operationsTestAuditor{}
	ctx := newContributionContext(owner, ServiceBag{Pages: renderer, OperationsAudit: auditor}, assetHostOptions{})
	return ctx, renderer, auditor
}

func validOperationsProvider() *operationsTestProvider {
	return &operationsTestProvider{
		state:     ServiceOperationsState{Maintenance: true, Progress: "Rotating snapshots"},
		snapshots: []BackupSnapshot{{ID: "snapshot-1", CreatedAt: time.Now(), SizeBytes: 42, Verification: VerificationVerified}},
		backupJob: OperationJob{ID: "backup-1", State: OperationQueued, Message: "Queued remotely"},
		restorePlan: RestorePlan{
			ID: "plan-1", Summary: "Restore one database", Warnings: []string{"Read-only mode required"}, ExpiresAt: time.Now().Add(time.Minute),
		},
		restoreJob: OperationJob{ID: "restore-1", State: OperationQueued, Message: "Waiting for maintenance window"},
	}
}

func TestOperationsRegistrationIsOwnerStampedAndAdminOnly(t *testing.T) {
	t.Parallel()

	ctx, _, _ := operationsTestContext("reports")
	require.NoError(t, ctx.Operations.Register(OperationsDescriptor{Label: "Reports"}, validOperationsProvider()))

	routes := ctx.Routes.All()
	require.Len(t, routes, 4)
	for _, route := range routes {
		assert.Equal(t, "reports", route.Owner)
		assert.Equal(t, GroupAdmin, route.Group)
		assert.True(t, strings.HasPrefix(route.Pattern, "/admin/operations/reports"))
		assert.NotContains(t, route.Pattern, "other-extension")
	}
}

func TestOperationsOverviewRequiresAdminAndRendersProviderState(t *testing.T) {
	t.Parallel()

	ctx, renderer, _ := operationsTestContext("reports")
	provider := validOperationsProvider()
	require.NoError(t, ctx.Operations.Register(OperationsDescriptor{Label: "Reports operations"}, provider))
	route, ok := ctx.Routes.Find(http.MethodGet, "/admin/operations/reports")
	require.True(t, ok)

	denied := httptest.NewRecorder()
	route.Handler.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, route.Pattern, nil))
	assert.Equal(t, http.StatusForbidden, denied.Code)

	request := withRequestContext(httptest.NewRequest(http.MethodGet, route.Pattern, nil), RequestContext{
		User: &RequestUser{ID: "admin-1", Username: "alex", IsAdmin: true}, CSRFToken: "csrf-token",
	})
	response := httptest.NewRecorder()
	route.Handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "admin_operations", renderer.page)
	assert.Equal(t, "reports", renderer.data.Owner)
	assert.True(t, renderer.data.State.Maintenance)
	require.Len(t, renderer.data.Snapshots, 1)
	assert.Equal(t, VerificationVerified, renderer.data.Snapshots[0].Verification)
}

func TestOperationsMutationsRequireCSRFAndUseDistinctAuditActions(t *testing.T) {
	t.Parallel()

	ctx, _, auditor := operationsTestContext("reports")
	provider := validOperationsProvider()
	require.NoError(t, ctx.Operations.Register(OperationsDescriptor{Label: "Reports"}, provider))
	backup, ok := ctx.Routes.Find(http.MethodPost, "/admin/operations/reports/backups")
	require.True(t, ok)

	missingCSRF := withRequestContext(httptest.NewRequest(http.MethodPost, backup.Pattern, nil), RequestContext{
		User: &RequestUser{ID: "admin-1", Username: "alex", IsAdmin: true}, CSRFToken: "csrf-token",
	})
	denied := httptest.NewRecorder()
	backup.Handler.ServeHTTP(denied, missingCSRF)
	assert.Equal(t, http.StatusForbidden, denied.Code)
	require.Len(t, auditor.events, 1)
	assert.Equal(t, ActionBackupCreate, auditor.events[0].Action)
	assert.Equal(t, "csrf_rejected", auditor.events[0].Outcome)

	response := invokeOperationsMutation(t, backup.Handler, backup.Pattern, url.Values{"csrf_token": {"csrf-token"}})
	assert.Equal(t, http.StatusOK, response.Code)
	require.Len(t, auditor.events, 2)
	assert.Equal(t, ActionBackupCreate, auditor.events[1].Action)
	assert.Equal(t, "reports", auditor.events[1].Extension)
	assert.Equal(t, "alex", auditor.events[1].Username)
	assert.Equal(t, "local/dev", provider.backupRequest.Build.Identity)
	assert.False(t, provider.backupRequest.RequestedAt.IsZero())
}

func TestRestoreRequiresValidatedPlanBoundToUserAndSecondConfirmation(t *testing.T) {
	t.Parallel()

	ctx, renderer, auditor := operationsTestContext("reports")
	provider := validOperationsProvider()
	require.NoError(t, ctx.Operations.Register(OperationsDescriptor{Label: "Reports"}, provider))
	plans, ok := ctx.Routes.Find(http.MethodPost, "/admin/operations/reports/restore-plans")
	require.True(t, ok)
	restores, ok := ctx.Routes.Find(http.MethodPost, "/admin/operations/reports/restores")
	require.True(t, ok)

	planned := invokeOperationsMutation(t, plans.Handler, plans.Pattern, url.Values{
		"csrf_token": {"csrf-token"}, "snapshot_id": {"snapshot-1"},
	})
	assert.Equal(t, http.StatusOK, planned.Code)
	require.NotNil(t, renderer.data.Plan)
	assert.Equal(t, "plan-1", renderer.data.Plan.ID)
	assert.Equal(t, "RESTORE reports", renderer.data.RestoreConfirmation)

	rejected := invokeOperationsMutation(t, restores.Handler, restores.Pattern, url.Values{
		"csrf_token": {"csrf-token"}, "plan_id": {"plan-1"}, "confirmation": {"yes"},
	})
	assert.Equal(t, http.StatusConflict, rejected.Code)
	assert.Empty(t, provider.executedPlan)

	accepted := invokeOperationsMutation(t, restores.Handler, restores.Pattern, url.Values{
		"csrf_token": {"csrf-token"}, "plan_id": {"plan-1"}, "confirmation": {"RESTORE reports"},
	})
	assert.Equal(t, http.StatusOK, accepted.Code)
	assert.Equal(t, "plan-1", provider.executedPlan)

	actions := make([]OperationAction, 0, len(auditor.events))
	for _, event := range auditor.events {
		actions = append(actions, event.Action)
	}
	assert.Contains(t, actions, ActionRestorePlan)
	assert.Contains(t, actions, ActionRestoreExecute)
}

func invokeOperationsMutation(t *testing.T, handler http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = withRequestContext(request, RequestContext{
		User: &RequestUser{ID: "admin-1", Username: "alex", IsAdmin: true}, CSRFToken: "csrf-token",
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
