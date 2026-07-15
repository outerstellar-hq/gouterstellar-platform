package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type mockPasswordResetRepo struct {
	mock.Mock
}

func (m *mockPasswordResetRepo) SavePasswordResetToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (db.PltPasswordResetToken, error) {
	args := m.Called(ctx, userID, token, expiresAt)
	return args.Get(0).(db.PltPasswordResetToken), args.Error(1)
}

func (m *mockPasswordResetRepo) FindByToken(ctx context.Context, token string) (db.PltPasswordResetToken, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(db.PltPasswordResetToken), args.Error(1)
}

func (m *mockPasswordResetRepo) MarkUsed(ctx context.Context, token string) (db.PltPasswordResetToken, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(db.PltPasswordResetToken), args.Error(1)
}

func TestResetPasswordUsesSharedPasswordPolicy(t *testing.T) {
	resetRepo := new(mockPasswordResetRepo)
	resetRepo.On("FindByToken", mock.Anything, "valid-token").Return(db.PltPasswordResetToken{
		ExpiresAt: pgtype.Timestamp{Time: time.Now().Add(time.Hour), Valid: true},
	}, nil)
	svc := NewPasswordResetService(nil, nil, resetRepo, nil, nil, "")

	err := svc.ResetPassword(context.Background(), "valid-token", "password1!")

	var weak *model.WeakPasswordError
	assert.ErrorAs(t, err, &weak)
	assert.ErrorContains(t, err, "uppercase")
	resetRepo.AssertExpectations(t)
}
