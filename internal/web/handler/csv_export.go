package handler

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
)

func writeCSV(w http.ResponseWriter, filename string, headers []string, rows [][]string) error {
	var body bytes.Buffer
	writer := csv.NewWriter(&body)
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, row := range rows {
		safeRow := make([]string, len(row))
		for i := range row {
			safeRow[i] = safeCSVCell(row[i])
		}
		if err := writer.Write(safeRow); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV: %w", err)
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := body.WriteTo(w); err != nil {
		return fmt.Errorf("send CSV: %w", err)
	}
	return nil
}

func safeCSVCell(value string) string {
	if value == "" || !strings.ContainsRune("=+-@\t\r\n", rune(value[0])) {
		return value
	}
	return "'" + value
}
