package model

import "fmt"

type ConflictStrategy int

const (
	ConflictMine ConflictStrategy = iota
	ConflictServer
)

func ConflictStrategyFromString(value string) ConflictStrategy {
	switch value {
	case "mine":
		return ConflictMine
	default:
		return ConflictServer
	}
}

type OuterstellarError struct {
	Message string
	Cause   error
}

func (e *OuterstellarError) Error() string { return e.Message }

type MessageNotFoundError struct {
	SyncID string
}

func (e *MessageNotFoundError) Error() string {
	return fmt.Sprintf("Message with sync ID %s was not found.", e.SyncID)
}

type ContactNotFoundError struct {
	SyncID string
}

func (e *ContactNotFoundError) Error() string {
	return fmt.Sprintf("Contact with sync ID %s was not found.", e.SyncID)
}

type DuplicateMessageError struct {
	SyncID string
}

func (e *DuplicateMessageError) Error() string {
	return fmt.Sprintf("A message with sync ID %s already exists.", e.SyncID)
}

type SyncConflictError struct {
	SyncID string
	Reason string
}

func (e *SyncConflictError) Error() string {
	return fmt.Sprintf("Sync conflict for message %s: %s", e.SyncID, e.Reason)
}

type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Validation failed: %v", e.Errors)
}

type OptimisticLockError struct {
	EntityType string
	SyncID     string
}

func (e *OptimisticLockError) Error() string {
	return fmt.Sprintf("%s with sync ID %s was modified by another process.", e.EntityType, e.SyncID)
}

type SyncError struct {
	Message string
	Cause   error
}

func (e *SyncError) Error() string { return e.Message }

type UsernameAlreadyExistsError struct {
	Username string
}

func (e *UsernameAlreadyExistsError) Error() string {
	return fmt.Sprintf("Username '%s' is already taken.", e.Username)
}

type WeakPasswordError struct {
	Message string
}

func (e *WeakPasswordError) Error() string { return e.Message }

type UserNotFoundError struct {
	UserID string
}

func (e *UserNotFoundError) Error() string {
	return fmt.Sprintf("User with ID %s was not found.", e.UserID)
}

type InsufficientPermissionError struct {
	Message string
}

func (e *InsufficientPermissionError) Error() string { return e.Message }

type SessionExpiredError struct{}

func (e *SessionExpiredError) Error() string { return "Session has expired" }
