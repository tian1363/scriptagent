package telemetry

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/tian1363/scriptagent"

var captureContent atomic.Bool
var environmentName atomic.Value
var releaseName atomic.Value

type Config struct {
	PublicKey      string
	SecretKey      string
	BaseURL        string
	Environment    string
	Release        string
	CaptureContent bool
}

type RunAttributes struct {
	Name      string
	RunID     string
	SpaceID   string
	JobID     string
	SessionID string
	Input     string
}

type GenerationAttributes struct {
	Name      string
	TraceName string
	RunID     string
	SpaceID   string
	RefID     string
	SessionID string
	Model     string
	Input     string
}

type EmbeddingAttributes = GenerationAttributes

// InitLangfuse configures an isolated OTLP/HTTP exporter when both project keys are present.
// Missing configuration disables external tracing without affecting local observability.
func InitLangfuse(ctx context.Context, cfg Config) (func(context.Context) error, bool, error) {
	publicKey := strings.TrimSpace(cfg.PublicKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if publicKey == "" && secretKey == "" {
		return func(context.Context) error { return nil }, false, nil
	}
	if publicKey == "" || secretKey == "" {
		return nil, false, errors.New("Langfuse requires both public and secret keys")
	}
	captureContent.Store(cfg.CaptureContent)
	environmentName.Store(strings.TrimSpace(cfg.Environment))
	releaseName.Store(strings.TrimSpace(cfg.Release))
	auth := base64.StdEncoding.EncodeToString([]byte(publicKey + ":" + secretKey))
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(langfuseTraceEndpoint(cfg.BaseURL)),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization":                "Basic " + auth,
			"x-langfuse-ingestion-version": "4",
		}),
		otlptracehttp.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, false, err
	}
	attrs := []attribute.KeyValue{semconv.ServiceName("scriptagent")}
	if value := strings.TrimSpace(cfg.Environment); value != "" {
		attrs = append(attrs, attribute.String("deployment.environment.name", value))
	}
	if value := strings.TrimSpace(cfg.Release); value != "" {
		attrs = append(attrs, attribute.String("service.version", value))
	}
	res, err := resource.New(ctx, resource.WithAttributes(attrs...))
	if err != nil {
		return nil, false, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(2*time.Second), sdktrace.WithExportTimeout(5*time.Second)),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, true, nil
}

func StartAgentRun(ctx context.Context, attrs RunAttributes) (context.Context, trace.Span) {
	name := valueOr(attrs.Name, "agent-run")
	spanAttrs := traceAttributes(name, attrs.RunID, attrs.SpaceID, attrs.JobID, attrs.SessionID)
	spanAttrs = append(spanAttrs,
		attribute.String("langfuse.observation.type", "agent"),
		attribute.String("langfuse.observation.input", Content(attrs.Input)),
	)
	return otel.Tracer(instrumentationName).Start(ctx, name, trace.WithAttributes(spanAttrs...))
}

func EndAgentRun(span trace.Span, output string, err error) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String("langfuse.observation.output", Content(output)))
	endSpan(span, err)
}

func StartGeneration(ctx context.Context, attrs GenerationAttributes) (context.Context, trace.Span) {
	name := valueOr(attrs.Name, "model-generation")
	spanAttrs := traceAttributes(valueOr(attrs.TraceName, "model-generation"), attrs.RunID, attrs.SpaceID, attrs.RefID, attrs.SessionID)
	spanAttrs = append(spanAttrs,
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("langfuse.observation.model.name", attrs.Model),
		attribute.String("langfuse.observation.input", Content(attrs.Input)),
		attribute.String("langfuse.observation.metadata.step", name),
	)
	return otel.Tracer(instrumentationName).Start(ctx, name, trace.WithAttributes(spanAttrs...))
}

func EndGeneration(span trace.Span, output string, inputTokens, outputTokens, totalTokens int, err error) {
	if span == nil {
		return
	}
	span.SetAttributes(
		attribute.String("langfuse.observation.output", Content(output)),
		attribute.String("langfuse.observation.usage_details", usageJSON(inputTokens, outputTokens, totalTokens)),
	)
	endSpan(span, err)
}

func StartEmbedding(ctx context.Context, attrs EmbeddingAttributes) (context.Context, trace.Span) {
	name := valueOr(attrs.Name, "embedding")
	spanAttrs := traceAttributes(valueOr(attrs.TraceName, "embedding"), attrs.RunID, attrs.SpaceID, attrs.RefID, attrs.SessionID)
	spanAttrs = append(spanAttrs,
		attribute.String("langfuse.observation.type", "embedding"),
		attribute.String("langfuse.observation.model.name", attrs.Model),
		attribute.String("langfuse.observation.input", Content(attrs.Input)),
		attribute.String("langfuse.observation.metadata.step", name),
	)
	return otel.Tracer(instrumentationName).Start(ctx, name, trace.WithAttributes(spanAttrs...))
}

func EndEmbedding(span trace.Span, vectorCount, totalTokens int, err error) {
	if span == nil {
		return
	}
	span.SetAttributes(
		attribute.Int("langfuse.observation.metadata.vector_count", vectorCount),
		attribute.String("langfuse.observation.usage_details", usageJSON(totalTokens, 0, totalTokens)),
	)
	endSpan(span, err)
}

func Content(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if captureContent.Load() {
		return value
	}
	return "[content capture disabled]"
}

func langfuseTraceEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}
	if strings.HasSuffix(baseURL, "/api/public/otel/v1/traces") {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/api/public/otel") {
		return baseURL + "/v1/traces"
	}
	return baseURL + "/api/public/otel/v1/traces"
}

func traceAttributes(traceName, runID, spaceID, refID, sessionID string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("langfuse.trace.name", traceName),
		attribute.String("langfuse.trace.metadata.run_id", runID),
		attribute.String("langfuse.trace.metadata.space_id", spaceID),
		attribute.String("langfuse.trace.metadata.ref_id", refID),
		attribute.StringSlice("langfuse.trace.tags", []string{"scriptagent"}),
	}
	if value, ok := environmentName.Load().(string); ok && value != "" {
		attrs = append(attrs, attribute.String("langfuse.environment", value))
	}
	if value, ok := releaseName.Load().(string); ok && value != "" {
		attrs = append(attrs, attribute.String("langfuse.release", value))
	}
	if sessionID != "" {
		attrs = append(attrs, attribute.String("langfuse.session.id", sessionID))
	}
	return attrs
}

func usageJSON(input, output, total int) string {
	return `{"input":` + itoa(input) + `,"output":` + itoa(output) + `,"total":` + itoa(total) + `}`
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	index := len(buf)
	for value > 0 {
		index--
		buf[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buf[index] = '-'
	}
	return string(buf[index:])
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("langfuse.observation.level", "ERROR"), attribute.String("langfuse.observation.status_message", err.Error()))
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
