package platform

import (
	"context"
	"io/fs"
	"net/http"
)

// MessageCounter is the capability for reading message counts without
// depending on internal service types.
type MessageCounter interface {
	CountMessages(ctx context.Context) (int64, error)
}

// PageRenderer is the public rendering capability used by extension page
// registries. Implementations validate and parse extension templates during
// startup, then render registered pages through the shared application shell.
type PageRenderer interface {
	RegisterTemplates(owner string, source fs.FS, pagesDir, partialsDir string) error
	RenderPage(w http.ResponseWriter, req *http.Request, page string, data any) error
}

// OperationsAuditor persists extension-operation audit events. Implementations
// must return write failures so a mutation is never reported as accepted when
// its audit record could not be produced.
type OperationsAuditor interface {
	RecordOperation(context.Context, OperationAudit) error
}

// ServiceBag carries platform-level capabilities that extensions can request.
// The wire root populates this by adapting internal services.
type ServiceBag struct {
	MessageCounter  MessageCounter
	Pages           PageRenderer
	OperationsAudit OperationsAuditor
}
