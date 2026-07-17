package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type stubMessageExportSource struct {
	list  func(int32, int32) (*model.PagedResult[model.MessageSummary], error)
	calls []int32
}

func (s *stubMessageExportSource) ListMessages(_ context.Context, limit, offset int32) (*model.PagedResult[model.MessageSummary], error) {
	s.calls = append(s.calls, offset)
	return s.list(limit, offset)
}

type stubContactExportSource struct {
	list  func(int32, int32) ([]model.ContactSummary, error)
	calls []int32
}

func (s *stubContactExportSource) ListContacts(_ context.Context, limit, offset int32) ([]model.ContactSummary, error) {
	s.calls = append(s.calls, offset)
	return s.list(limit, offset)
}

func TestAdminAPIRoutesRejectNonAdmins(t *testing.T) {
	contributors := []interface {
		ContributeRoutes(*extplatform.ContributionContext) error
	}{
		NewUserAdminAPI(nil),
		NewDataExportHandler(nil, nil),
	}

	for _, contributor := range contributors {
		ctx := extplatform.NewContributionContext("platform-core")
		require.NoError(t, contributor.ContributeRoutes(ctx))
		for _, route := range ctx.Routes.All() {
			t.Run(route.Pattern+"/anonymous", func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(route.Method, route.Pattern, nil)
				route.Handler.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusUnauthorized, recorder.Code)
			})
			t.Run(route.Pattern+"/user", func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(route.Method, route.Pattern, nil)
				request = web.WithUser(request, &model.User{Role: model.RoleUser})
				route.Handler.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusForbidden, recorder.Code)
			})
		}
	}
}

func TestAdminJSONDownloadUsesMinimalShapeAndPrivateHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := writeJSONDownload(recorder, "users.json", []userJSONExportRow{{
		Username: "alice", Email: "alice@example.com", Role: "ADMIN", Enabled: true,
	}})

	require.NoError(t, err)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="users.json"`, recorder.Header().Get("Content-Disposition"))
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.JSONEq(t, `[{"username":"alice","email":"alice@example.com","role":"ADMIN","enabled":true}]`, recorder.Body.String())
	assert.NotContains(t, recorder.Body.String(), "password")
	assert.NotContains(t, recorder.Body.String(), "locked")
}

func TestAdminUIRegistersJSONExportRoutes(t *testing.T) {
	ctx := extplatform.NewContributionContext("platform-core")
	require.NoError(t, NewUserAdminHandler(nil, nil).ContributeRoutes(ctx))

	patterns := make([]string, 0, len(ctx.Routes.All()))
	for _, route := range ctx.Routes.All() {
		patterns = append(patterns, route.Pattern)
	}
	assert.Contains(t, patterns, "/admin/users/export/json")
	assert.Contains(t, patterns, "/admin/audit/export/json")
}

func TestAdminAPIRegistersJavaCompatibleRoutes(t *testing.T) {
	ctx := extplatform.NewContributionContext("platform-core")
	require.NoError(t, NewUserAdminAPI(nil).ContributeRoutes(ctx))

	routes := make(map[string]bool)
	for _, route := range ctx.Routes.All() {
		routes[route.Method+" "+route.Pattern] = true
	}
	assert.True(t, routes[http.MethodGet+" /api/v1/admin/users"])
	assert.True(t, routes[http.MethodPut+" /api/v1/admin/users/{id}/enabled"])
	assert.True(t, routes[http.MethodPut+" /api/v1/admin/users/{id}/role"])
}

func TestAdminPaginationAcceptsJavaLimitOffsetContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/admin/users?limit=250&offset=205", nil)

	limit, offset, page := adminPagination(request)

	assert.Equal(t, adminMaxPageSize, limit)
	assert.Equal(t, 205, offset)
	assert.Equal(t, 3, page)
}

func TestMessageCSVExportPaginatesAndNeutralizesFormulas(t *testing.T) {
	firstPage := make([]model.MessageSummary, exportPageSize)
	firstPage[0] = model.MessageSummary{
		Author:           "=HYPERLINK(\"https://example.test\")",
		Content:          "+SUM(1,1)",
		UpdatedAtEpochMs: 0,
		Dirty:            true,
	}
	messages := &stubMessageExportSource{list: func(_ int32, offset int32) (*model.PagedResult[model.MessageSummary], error) {
		if offset == 0 {
			return &model.PagedResult[model.MessageSummary]{Items: firstPage}, nil
		}
		return &model.PagedResult[model.MessageSummary]{Items: []model.MessageSummary{{Author: "last", Content: "page"}}}, nil
	}}
	handler := NewDataExportHandler(messages, &stubContactExportSource{})
	recorder := httptest.NewRecorder()

	handler.ExportMessagesCSV(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/export/message/csv", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, `attachment; filename="message.csv"`, recorder.Header().Get("Content-Disposition"))
	records, err := csv.NewReader(strings.NewReader(recorder.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, int(exportPageSize)+2)
	assert.Equal(t, []string{"Author", "Content", "Updated", "Dirty"}, records[0])
	assert.Equal(t, "'=HYPERLINK(\"https://example.test\")", records[1][0])
	assert.Equal(t, "'+SUM(1,1)", records[1][1])
	assert.Equal(t, "1970-01-01", records[1][2])
	assert.Equal(t, []int32{0, exportPageSize}, messages.calls)
}

func TestJSONExportsMatchOriginalContracts(t *testing.T) {
	messages := &stubMessageExportSource{list: func(_, _ int32) (*model.PagedResult[model.MessageSummary], error) {
		return &model.PagedResult[model.MessageSummary]{Items: []model.MessageSummary{{
			Author: "alice", Content: "hello", UpdatedAtEpochMs: 1234, Dirty: true,
		}}}, nil
	}}
	contacts := &stubContactExportSource{list: func(_, _ int32) ([]model.ContactSummary, error) {
		return []model.ContactSummary{{
			Name: "Bob", Emails: nil, Phones: []string{"+40 123"}, Company: "Acme", Department: "Ops",
		}}, nil
	}}
	handler := NewDataExportHandler(messages, contacts)

	messageRecorder := httptest.NewRecorder()
	handler.ExportMessagesJSON(messageRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/export/message/json", nil))
	assert.JSONEq(t, `[{"author":"alice","content":"hello","updated":1234,"dirty":true}]`, messageRecorder.Body.String())

	contactRecorder := httptest.NewRecorder()
	handler.ExportContactsJSON(contactRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/export/contact/json", nil))
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(contactRecorder.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, []any{}, rows[0]["emails"])
	assert.NotContains(t, rows[0], "phones", "the original contact JSON contract does not include phones")
}

func TestContactCSVExportUsesOriginalColumns(t *testing.T) {
	contacts := &stubContactExportSource{list: func(_, _ int32) ([]model.ContactSummary, error) {
		return []model.ContactSummary{{
			Name: "Alice", Emails: []string{"a@example.test", "b@example.test"}, Phones: []string{"+40 123"}, Company: "@Acme", Department: "R&D",
		}}, nil
	}}
	handler := NewDataExportHandler(&stubMessageExportSource{}, contacts)
	recorder := httptest.NewRecorder()

	handler.ExportContactsCSV(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/export/contact/csv", nil))

	records, err := csv.NewReader(strings.NewReader(recorder.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, []string{"Name", "Emails", "Phones", "Company", "Department"}, records[0])
	assert.Equal(t, []string{"Alice", "a@example.test; b@example.test", "'+40 123", "'@Acme", "R&D"}, records[1])
}

func TestSharedCSVWriterNeutralizesCellsWithoutMutatingRows(t *testing.T) {
	rows := [][]string{{"=formula", "+phone", "plain"}}
	recorder := httptest.NewRecorder()

	require.NoError(t, writeCSV(recorder, "test.csv", []string{"A", "B", "C"}, rows))

	records, err := csv.NewReader(strings.NewReader(recorder.Body.String())).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"'=formula", "'+phone", "plain"}, records[1])
	assert.Equal(t, []string{"=formula", "+phone", "plain"}, rows[0])
}

func TestExportFailureDoesNotReturnPartialDownload(t *testing.T) {
	firstPage := make([]model.MessageSummary, exportPageSize)
	messages := &stubMessageExportSource{list: func(_ int32, offset int32) (*model.PagedResult[model.MessageSummary], error) {
		if offset == 0 {
			return &model.PagedResult[model.MessageSummary]{Items: firstPage}, nil
		}
		return nil, errors.New("database unavailable")
	}}
	handler := NewDataExportHandler(messages, nil)
	recorder := httptest.NewRecorder()

	handler.ExportMessagesJSON(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/export/message/json", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Empty(t, recorder.Header().Get("Content-Disposition"))
	assert.JSONEq(t, `{"error":"Internal server error"}`, recorder.Body.String())
}
