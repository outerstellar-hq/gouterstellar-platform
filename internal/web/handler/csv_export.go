package handler

import (
	"encoding/csv"
	"net/http"
	"strings"
)

func writeCSV(w http.ResponseWriter, filename string, headers []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	writer := csv.NewWriter(w)
	_ = writer.Write(safeCSVRow(headers))
	for _, row := range rows {
		_ = writer.Write(safeCSVRow(row))
	}
	writer.Flush()
}

func safeCSVRow(row []string) []string {
	safe := make([]string, len(row))
	for i, value := range row {
		trimmed := strings.TrimLeft(value, " \t\r\n")
		if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
			safe[i] = "'" + value
		} else {
			safe[i] = value
		}
	}
	return safe
}
