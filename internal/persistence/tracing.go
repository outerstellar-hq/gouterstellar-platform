package persistence

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// queryTracer implements pgx.QueryTracer to record the duration of every
// Query, QueryRow, and Exec call made through a pooled connection. Each
// completed query is emitted as a structured debug log line carrying the
// (truncated) SQL text and its elapsed time in milliseconds. This gives the
// platform low-overhead database observability without pulling in the full
// OpenTelemetry pgx integration.
type queryTracer struct{}

// NewTracingTracer returns a pgx.QueryTracer that logs query durations.
// Attach it to a pool via config.ConnConfig.Tracer before constructing the
// pool with pgxpool.NewWithConfig.
func NewTracingTracer() pgx.QueryTracer {
	return &queryTracer{}
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTraceKey{}, queryTrace{
		sql:   data.SQL,
		start: time.Now(),
	})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	tr, ok := ctx.Value(queryTraceKey{}).(queryTrace)
	if !ok {
		return
	}
	elapsed := time.Since(tr.start)
	if data.Err != nil {
		slog.Debug("database query failed",
			"sql", truncateSQL(tr.sql, 100),
			"duration_ms", elapsed.Milliseconds(),
			"error", data.Err,
		)
		return
	}
	slog.Debug("database query",
		"sql", truncateSQL(tr.sql, 100),
		"duration_ms", elapsed.Milliseconds(),
	)
}

type queryTrace struct {
	sql   string
	start time.Time
}

type queryTraceKey struct{}

// truncateSQL caps the logged SQL text so a single oversized statement does
// not bloat structured log output.
func truncateSQL(sql string, max int) string {
	if len(sql) <= max {
		return sql
	}
	return sql[:max] + "..."
}
