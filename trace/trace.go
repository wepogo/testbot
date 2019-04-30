package trace

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// StartSpanFromEnv returns a new root span
// with its trace ID and parent span ID set from
// environment variable $SPAN.
// If $SPAN is empty, it returns nil.
func StartSpanFromEnv(name, service, resource string) (ddtrace.Span, error) {
	s := os.Getenv("SPAN")
	if s == "" {
		return nil, nil
	}
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return nil, fmt.Errorf("$SPAN missing colon")
	}
	traceID, err := strconv.ParseUint(s[:i], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("$SPAN trace ID: %v", err)
	}
	parentID, err := strconv.ParseUint(s[i+1:], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("$SPAN parent ID: %v", err)
	}

	var c tracer.TextMapCarrier
	c = map[string]string{}
	c.Set(tracer.DefaultTraceIDHeader, string(traceID))
	c.Set(tracer.DefaultParentIDHeader, string(parentID))
	ctx, _ := tracer.Extract(c)

	span := tracer.StartSpan(name,
		tracer.ChildOf(ctx),
		tracer.ServiceName(service),
		tracer.ResourceName(resource),
	)
	return span, nil
}

// EnvironmentFor formats the span in ctx
// as one or more environment list entries
// suitable for use by StartSpanFromEnv.
func EnvironmentFor(ctx context.Context) []string {
	span, ok := tracer.SpanFromContext(ctx)
	if !ok {
		return nil
	}
	return []string{fmt.Sprintf("SPAN=%d:%d", span.Context().TraceID(), span.Context().SpanID())}
}
