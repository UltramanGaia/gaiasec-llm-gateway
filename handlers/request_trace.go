package handlers

import (
	"context"
	"net/http"
	"strings"

	"llm-gateway/utils"

	log "github.com/sirupsen/logrus"
)

type requestTraceKey struct{}

func requestTraceIDFromHeaders(headers http.Header) string {
	if headers == nil {
		return utils.GenerateID()
	}

	for _, key := range []string{"X-Trace-ID", "X-Request-ID", "X-Correlation-ID"} {
		if value := strings.TrimSpace(headers.Get(key)); value != "" {
			return value
		}
	}

	return utils.GenerateID()
}

func withRequestTrace(ctx context.Context, traceID string) context.Context {
	if strings.TrimSpace(traceID) == "" {
		return ctx
	}
	return context.WithValue(ctx, requestTraceKey{}, traceID)
}

func requestTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceID, _ := ctx.Value(requestTraceKey{}).(string)
	return strings.TrimSpace(traceID)
}

func loggerWithTrace(ctx context.Context) *log.Entry {
	traceID := requestTraceID(ctx)
	if traceID == "" {
		return log.NewEntry(log.StandardLogger())
	}
	return log.WithField("trace_id", traceID)
}

func fieldsWithTrace(ctx context.Context, fields log.Fields) log.Fields {
	out := make(log.Fields, len(fields)+1)
	for key, value := range fields {
		out[key] = value
	}
	if traceID := requestTraceID(ctx); traceID != "" {
		out["trace_id"] = traceID
	}
	return out
}

func logContextState(ctx context.Context, fields log.Fields, message string) {
	if ctx == nil || ctx.Err() == nil {
		return
	}

	entry := log.WithFields(fieldsWithTrace(ctx, fields)).WithField("context_err", ctx.Err().Error())
	if cause := context.Cause(ctx); cause != nil {
		entry = entry.WithField("context_cause", cause.Error())
	}
	entry.Warn(message)
}
