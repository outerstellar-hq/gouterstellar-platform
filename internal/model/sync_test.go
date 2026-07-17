package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncPushValidationMatchesJavaLimits(t *testing.T) {
	require.NoError(t, (SyncPushRequest{Messages: []SyncMessage{{
		SyncID: "id", Author: strings.Repeat("a", SyncMaxAuthorLength), Content: strings.Repeat("c", SyncMaxContentLength),
	}}}).Validate())

	tests := []SyncMessage{
		{SyncID: "", Author: "author", Content: "content"},
		{SyncID: "id", Author: " ", Content: "content"},
		{SyncID: "id", Author: "author", Content: "\t"},
		{SyncID: "id", Author: strings.Repeat("a", SyncMaxAuthorLength+1), Content: "content"},
		{SyncID: "id", Author: "author", Content: strings.Repeat("c", SyncMaxContentLength+1)},
	}
	for _, message := range tests {
		require.Error(t, (SyncPushRequest{Messages: []SyncMessage{message}}).Validate())
	}
}

func TestSyncContactValidationMatchesJavaLimits(t *testing.T) {
	require.NoError(t, (SyncPushContactRequest{Contacts: []SyncContact{{
		SyncID: "id", Name: strings.Repeat("n", SyncMaxNameLength),
	}}}).Validate())

	tests := []SyncContact{
		{SyncID: "", Name: "name"},
		{SyncID: "id", Name: " "},
		{SyncID: "id", Name: strings.Repeat("n", SyncMaxNameLength+1)},
		{SyncID: "id", Name: "name", Emails: []string{strings.Repeat("e", SyncMaxEmailLength+1)}},
		{SyncID: "id", Name: "name", Phones: []string{strings.Repeat("p", SyncMaxPhoneLength+1)}},
		{SyncID: "id", Name: "name", SocialMedia: []string{strings.Repeat("s", SyncMaxSocialLength+1)}},
	}
	for _, contact := range tests {
		require.Error(t, (SyncPushContactRequest{Contacts: []SyncContact{contact}}).Validate())
	}
}
