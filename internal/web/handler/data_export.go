package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	extplatform "github.com/rygel/gouterstellar-platform/platform"

	"github.com/rygel/gouterstellar-platform/internal/model"
)

const exportPageSize int32 = 500

type messageExportSource interface {
	ListMessages(context.Context, int32, int32) (*model.PagedResult[model.MessageSummary], error)
}

type contactExportSource interface {
	ListContacts(context.Context, int32, int32) ([]model.ContactSummary, error)
}

// DataExportHandler serves the admin-only message and contact export API.
type DataExportHandler struct {
	messages messageExportSource
	contacts contactExportSource
}

// NewDataExportHandler constructs the platform data export API.
func NewDataExportHandler(messages messageExportSource, contacts contactExportSource) *DataExportHandler {
	return &DataExportHandler{messages: messages, contacts: contacts}
}

// ContributeRoutes registers bearer-authenticated, admin-only export routes.
func (h *DataExportHandler) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	routes := []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/api/v1/admin/export/message/csv", h.ExportMessagesCSV},
		{"/api/v1/admin/export/message/json", h.ExportMessagesJSON},
		{"/api/v1/admin/export/contact/csv", h.ExportContactsCSV},
		{"/api/v1/admin/export/contact/json", h.ExportContactsJSON},
	}
	for _, route := range routes {
		ctx.Routes.API(http.MethodGet, route.path, "Export platform data", requireAdminAPI(route.handler))
	}
	return nil
}

// ExportMessagesCSV downloads all active messages in the original CSV shape.
func (h *DataExportHandler) ExportMessagesCSV(w http.ResponseWriter, r *http.Request) {
	h.serveExport(w, r, "message.csv", "text/csv; charset=utf-8", h.writeMessagesCSV)
}

// ExportMessagesJSON downloads all active messages in the original JSON shape.
func (h *DataExportHandler) ExportMessagesJSON(w http.ResponseWriter, r *http.Request) {
	h.serveExport(w, r, "message.json", "application/json; charset=utf-8", h.writeMessagesJSON)
}

// ExportContactsCSV downloads all active contacts in the original CSV shape.
func (h *DataExportHandler) ExportContactsCSV(w http.ResponseWriter, r *http.Request) {
	h.serveExport(w, r, "contact.csv", "text/csv; charset=utf-8", h.writeContactsCSV)
}

// ExportContactsJSON downloads all active contacts in the original JSON shape.
func (h *DataExportHandler) ExportContactsJSON(w http.ResponseWriter, r *http.Request) {
	h.serveExport(w, r, "contact.json", "application/json; charset=utf-8", h.writeContactsJSON)
}

func (h *DataExportHandler) serveExport(w http.ResponseWriter, r *http.Request, filename, contentType string, write func(context.Context, io.Writer) error) {
	file, err := os.CreateTemp("", "outerstellar-export-*")
	if err != nil {
		handleServiceError(w, fmt.Errorf("create export staging file: %w", err))
		return
	}
	path := file.Name()
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("Failed to close export staging file", "error", err)
		}
		if err := os.Remove(path); err != nil {
			slog.Error("Failed to remove export staging file", "error", err)
		}
	}()

	if err := write(r.Context(), file); err != nil {
		handleServiceError(w, err)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		handleServiceError(w, fmt.Errorf("rewind export: %w", err))
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, filename, time.Time{}, file)
}

func (h *DataExportHandler) writeMessagesCSV(ctx context.Context, out io.Writer) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{"Author", "Content", "Updated", "Dirty"}); err != nil {
		return fmt.Errorf("write message CSV header: %w", err)
	}
	err := h.eachMessagePage(ctx, func(messages []model.MessageSummary) error {
		for _, message := range messages {
			row := []string{
				safeCSVCell(message.Author),
				safeCSVCell(message.Content),
				time.UnixMilli(message.UpdatedAtEpochMs).UTC().Format(time.DateOnly),
				strconv.FormatBool(message.Dirty),
			}
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("write message CSV row: %w", err)
			}
		}
		return nil
	})
	writer.Flush()
	if err != nil {
		return err
	}
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush message CSV: %w", err)
	}
	return nil
}

func (h *DataExportHandler) writeContactsCSV(ctx context.Context, out io.Writer) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{"Name", "Emails", "Phones", "Company", "Department"}); err != nil {
		return fmt.Errorf("write contact CSV header: %w", err)
	}
	err := h.eachContactPage(ctx, func(contacts []model.ContactSummary) error {
		for _, contact := range contacts {
			row := []string{
				safeCSVCell(contact.Name),
				safeCSVCell(strings.Join(contact.Emails, "; ")),
				safeCSVCell(strings.Join(contact.Phones, "; ")),
				safeCSVCell(contact.Company),
				safeCSVCell(contact.Department),
			}
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("write contact CSV row: %w", err)
			}
		}
		return nil
	})
	writer.Flush()
	if err != nil {
		return err
	}
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush contact CSV: %w", err)
	}
	return nil
}

type messageExportRow struct {
	Author  string `json:"author"`
	Content string `json:"content"`
	Updated int64  `json:"updated"`
	Dirty   bool   `json:"dirty"`
}

func (h *DataExportHandler) writeMessagesJSON(ctx context.Context, out io.Writer) error {
	return writeJSONArray(out, func(encode func(any) error) error {
		return h.eachMessagePage(ctx, func(messages []model.MessageSummary) error {
			for _, message := range messages {
				if err := encode(messageExportRow{message.Author, message.Content, message.UpdatedAtEpochMs, message.Dirty}); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

type contactExportRow struct {
	Name       string   `json:"name"`
	Emails     []string `json:"emails"`
	Company    string   `json:"company"`
	Department string   `json:"department"`
}

func (h *DataExportHandler) writeContactsJSON(ctx context.Context, out io.Writer) error {
	return writeJSONArray(out, func(encode func(any) error) error {
		return h.eachContactPage(ctx, func(contacts []model.ContactSummary) error {
			for _, contact := range contacts {
				emails := contact.Emails
				if emails == nil {
					emails = []string{}
				}
				if err := encode(contactExportRow{contact.Name, emails, contact.Company, contact.Department}); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func writeJSONArray(out io.Writer, rows func(func(any) error) error) error {
	if _, err := io.WriteString(out, "["); err != nil {
		return fmt.Errorf("write JSON export: %w", err)
	}
	first := true
	encode := func(row any) error {
		if !first {
			if _, err := io.WriteString(out, ","); err != nil {
				return fmt.Errorf("write JSON export separator: %w", err)
			}
		}
		first = false
		encoded, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode JSON export row: %w", err)
		}
		if _, err := out.Write(encoded); err != nil {
			return fmt.Errorf("write JSON export row: %w", err)
		}
		return nil
	}
	if err := rows(encode); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "]"); err != nil {
		return fmt.Errorf("finish JSON export: %w", err)
	}
	return nil
}

func (h *DataExportHandler) eachMessagePage(ctx context.Context, visit func([]model.MessageSummary) error) error {
	for offset := int64(0); ; offset += int64(exportPageSize) {
		if offset > math.MaxInt32 {
			return fmt.Errorf("message export exceeds supported pagination range")
		}
		page, err := h.messages.ListMessages(ctx, exportPageSize, int32(offset))
		if err != nil {
			return fmt.Errorf("list messages for export: %w", err)
		}
		if err := visit(page.Items); err != nil {
			return err
		}
		if len(page.Items) < int(exportPageSize) {
			return nil
		}
	}
}

func (h *DataExportHandler) eachContactPage(ctx context.Context, visit func([]model.ContactSummary) error) error {
	for offset := int64(0); ; offset += int64(exportPageSize) {
		if offset > math.MaxInt32 {
			return fmt.Errorf("contact export exceeds supported pagination range")
		}
		page, err := h.contacts.ListContacts(ctx, exportPageSize, int32(offset))
		if err != nil {
			return fmt.Errorf("list contacts for export: %w", err)
		}
		if err := visit(page); err != nil {
			return err
		}
		if len(page) < int(exportPageSize) {
			return nil
		}
	}
}
