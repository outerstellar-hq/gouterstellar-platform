package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type deviceTokenRepoStub struct {
	deletedToken string
	deletedUser  uuid.UUID
	deleteResult *int64
}

func (s *deviceTokenRepoStub) UpsertDeviceToken(context.Context, uuid.UUID, string, string, *string) (db.PltDeviceToken, error) {
	return db.PltDeviceToken{}, nil
}

func (s *deviceTokenRepoStub) DeleteDeviceToken(context.Context, int64, uuid.UUID) (int64, error) {
	return 0, nil
}

func (s *deviceTokenRepoStub) DeleteDeviceTokenByValue(_ context.Context, token string, userID uuid.UUID) (int64, error) {
	s.deletedToken = token
	s.deletedUser = userID
	if s.deleteResult != nil {
		return *s.deleteResult, nil
	}
	return 1, nil
}

func (s *deviceTokenRepoStub) FindByUserID(context.Context, uuid.UUID) ([]db.PltDeviceToken, error) {
	return nil, nil
}

func (s *deviceTokenRepoStub) DeleteAllForUser(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func TestUnregisterDeviceByTokenMatchesJavaContract(t *testing.T) {
	repo := &deviceTokenRepoStub{}
	h := NewDeviceRegistrationAPI(repo)
	user := &model.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/register", strings.NewReader(`{"token":"push-token"}`))
	req = web.WithUser(req, user)
	recorder := httptest.NewRecorder()

	h.UnregisterByToken(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "push-token", repo.deletedToken)
	assert.Equal(t, user.ID, repo.deletedUser)
}

func TestUnregisterDeviceByTokenRejectsUnknownOwnership(t *testing.T) {
	zero := int64(0)
	repo := &deviceTokenRepoStub{deleteResult: &zero}
	h := NewDeviceRegistrationAPI(repo)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/register?token=missing", nil)
	req = web.WithUser(req, &model.User{ID: uuid.New()})
	recorder := httptest.NewRecorder()

	h.UnregisterByToken(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestRegisterDeviceMatchesJavaPlatformAndStatusContract(t *testing.T) {
	h := NewDeviceRegistrationAPI(&deviceTokenRepoStub{})
	user := &model.User{ID: uuid.New()}

	valid := httptest.NewRequest(http.MethodPost, "/api/v1/devices/register", strings.NewReader(`{"platform":"android","token":"push-token"}`))
	valid = web.WithUser(valid, user)
	validRecorder := httptest.NewRecorder()
	h.Register(validRecorder, valid)
	assert.Equal(t, http.StatusNoContent, validRecorder.Code)

	invalid := httptest.NewRequest(http.MethodPost, "/api/v1/devices/register", strings.NewReader(`{"platform":"web","token":"push-token"}`))
	invalid = web.WithUser(invalid, user)
	invalidRecorder := httptest.NewRecorder()
	h.Register(invalidRecorder, invalid)
	assert.Equal(t, http.StatusBadRequest, invalidRecorder.Code)
}
