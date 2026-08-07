package log

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWithContextInjectsTraceFields(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	Logger = zap.New(core).With(zap.String("service", "reward-mservice")).Sugar()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		Logger = zap.NewNop().Sugar()
	})

	ctx, span := otel.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	WithContext(ctx).Infow("hello")
	logs := recorded.All()
	assert.Len(t, logs, 1)

	fields := logs[0].ContextMap()
	assert.Equal(t, "reward-mservice", fields["service"])
	assert.Equal(t, span.SpanContext().TraceID().String(), fields["trace_id"])
	assert.Equal(t, span.SpanContext().SpanID().String(), fields["span_id"])
}

func TestWithContextAlwaysHasTraceKeys(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	Logger = zap.New(core).With(zap.String("service", "reward-mservice")).Sugar()
	t.Cleanup(func() {
		Logger = zap.NewNop().Sugar()
	})

	WithContext(context.Background()).Infow("no-active-span")
	logs := recorded.All()
	assert.Len(t, logs, 1)
	fields := logs[0].ContextMap()
	assert.Equal(t, "reward-mservice", fields["service"])
	assert.Contains(t, fields, "trace_id")
	assert.Contains(t, fields, "span_id")
	assert.Equal(t, "", fields["trace_id"])
	assert.Equal(t, "", fields["span_id"])
}

func TestHTTPResponseIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx, span := otel.Tracer("test").Start(c.Request.Context(), "http")
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(HTTPResponseIDMiddleware())
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(RequestIDHeader, "req-fixed")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "req-fixed", w.Header().Get(RequestIDHeader))
	assert.NotEmpty(t, w.Header().Get(TraceIDHeader))
}
