package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/google/uuid"
)

// mockSourceRepository for testing.
type mockSourceRepository struct {
	source *domain.Source
	err    error
}

func (m *mockSourceRepository) Create(_ context.Context, _ *domain.Source) error {
	return nil
}

func (m *mockSourceRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.Source, error) {
	return m.source, m.err
}

func (m *mockSourceRepository) GetByAPIKey(_ context.Context, _ string) (*domain.Source, error) {
	return m.source, m.err
}

func (m *mockSourceRepository) GetByName(_ context.Context, _ string) (*domain.Source, error) {
	return m.source, m.err
}

// mockPipeline for testing.
type mockPipeline struct {
	enqueueErr error
}

func (m *mockPipeline) Enqueue(_ *domain.Event) error {
	return m.enqueueErr
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func newRequest(body interface{}) *http.Request {
	var bodyReader io.Reader
	switch v := body.(type) {
	case []byte:
		bodyReader = bytes.NewReader(v)
	case string:
		bodyReader = strings.NewReader(v)
	default:
		bodyReader = bytes.NewReader(nil)
	}
	ctx := context.Background()
	return httptest.NewRequestWithContext(ctx, "POST", "/v1/events", bodyReader)
}

func TestHandleIngestEvent_SuccessValidEvent(t *testing.T) {
	logger := newTestLogger()
	source := &domain.Source{
		ID:     uuid.New(),
		Name:   "test-shop",
		APIKey: "sk-test-123",
	}
	mockRepo := &mockSourceRepository{source: source}
	mockPipe := &mockPipeline{}
	gw := NewGateway(logger, mockPipe, mockRepo)

	body := IngestEventRequest{
		EventID:   "evt-001",
		EventType: "purchase",
		Source:    "test-shop",
		Payload:   json.RawMessage(`{"price": 120}`),
	}
	bodyBytes, _ := json.Marshal(body)

	req := newRequest(bodyBytes)
	req.Header.Set("X-Api-Key", "sk-test-123")
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp successResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Event accepted", resp.Message)
}

func TestHandleIngestEvent_ErrorMissingEventType(t *testing.T) {
	logger := newTestLogger()
	source := &domain.Source{
		ID:     uuid.New(),
		Name:   "test-shop",
		APIKey: "sk-test-123",
	}
	mockRepo := &mockSourceRepository{source: source}
	mockPipe := &mockPipeline{}
	gw := NewGateway(logger, mockPipe, mockRepo)

	body := IngestEventRequest{
		EventID: "evt-001",
		// Missing EventType
		Source:  "test-shop",
		Payload: json.RawMessage(`{}`),
	}
	bodyBytes, _ := json.Marshal(body)

	req := newRequest(bodyBytes)
	req.Header.Set("X-Api-Key", "sk-test-123")
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp errorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_ARGUMENT", resp.Code)
}

func TestHandleIngestEvent_ErrorInvalidJSON(t *testing.T) {
	logger := newTestLogger()
	source := &domain.Source{
		ID:     uuid.New(),
		Name:   "test-shop",
		APIKey: "sk-test-123",
	}
	mockRepo := &mockSourceRepository{source: source}
	mockPipe := &mockPipeline{}
	gw := NewGateway(logger, mockPipe, mockRepo)

	req := newRequest("invalid json {")
	req.Header.Set("X-Api-Key", "sk-test-123")
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp errorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_JSON", resp.Code)
}

func TestHandleIngestEvent_ErrorMissingAPIKey(t *testing.T) {
	logger := newTestLogger()
	mockRepo := &mockSourceRepository{}
	mockPipe := &mockPipeline{}
	gw := NewGateway(logger, mockPipe, mockRepo)

	body := IngestEventRequest{
		EventID:   "evt-001",
		EventType: "purchase",
		Source:    "test-shop",
		Payload:   json.RawMessage(`{}`),
	}
	bodyBytes, _ := json.Marshal(body)

	req := newRequest(bodyBytes)
	// No X-Api-Key header
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp errorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "UNAUTHENTICATED", resp.Code)
}

func TestHandleIngestEvent_ErrorUnknownAPIKey(t *testing.T) {
	logger := newTestLogger()
	mockRepo := &mockSourceRepository{err: errors.New("source not found")}
	mockPipe := &mockPipeline{}
	gw := NewGateway(logger, mockPipe, mockRepo)

	body := IngestEventRequest{
		EventID:   "evt-001",
		EventType: "purchase",
		Source:    "test-shop",
		Payload:   json.RawMessage(`{}`),
	}
	bodyBytes, _ := json.Marshal(body)

	req := newRequest(bodyBytes)
	req.Header.Set("X-Api-Key", "sk-invalid-key")
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp errorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "UNAUTHENTICATED", resp.Code)
}

func TestHandleIngestEvent_ErrorSourceMismatch(t *testing.T) {
	logger := newTestLogger()
	source := &domain.Source{
		ID:     uuid.New(),
		Name:   "test-shop",
		APIKey: "sk-test-123",
	}
	mockRepo := &mockSourceRepository{source: source}
	mockPipe := &mockPipeline{}
	gw := NewGateway(logger, mockPipe, mockRepo)

	body := IngestEventRequest{
		EventID:   "evt-001",
		EventType: "purchase",
		Source:    "other-shop", // Mismatch: API key is for test-shop
		Payload:   json.RawMessage(`{}`),
	}
	bodyBytes, _ := json.Marshal(body)

	req := newRequest(bodyBytes)
	req.Header.Set("X-Api-Key", "sk-test-123")
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp errorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "PERMISSION_DENIED", resp.Code)
}

func TestHandleIngestEvent_ErrorPipelineFull(t *testing.T) {
	logger := newTestLogger()
	source := &domain.Source{
		ID:     uuid.New(),
		Name:   "test-shop",
		APIKey: "sk-test-123",
	}
	mockRepo := &mockSourceRepository{source: source}
	mockPipe := &mockPipeline{enqueueErr: engine.ErrPipelineFull}
	gw := NewGateway(logger, mockPipe, mockRepo)

	body := IngestEventRequest{
		EventID:   "evt-001",
		EventType: "purchase",
		Source:    "test-shop",
		Payload:   json.RawMessage(`{}`),
	}
	bodyBytes, _ := json.Marshal(body)

	req := newRequest(bodyBytes)
	req.Header.Set("X-Api-Key", "sk-test-123")
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp errorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "RESOURCE_EXHAUSTED", resp.Code)
}

func TestHandleIngestEvent_ErrorBodyTooLarge(t *testing.T) {
	logger := newTestLogger()
	source := &domain.Source{
		ID:     uuid.New(),
		Name:   "test-shop",
		APIKey: "sk-test-123",
	}
	mockRepo := &mockSourceRepository{source: source}
	mockPipe := &mockPipeline{}
	gw := NewGateway(logger, mockPipe, mockRepo)

	// Create a body larger than 1 MB
	largePayload := make([]byte, maxBodyBytes+1)
	for i := range largePayload {
		largePayload[i] = 'a'
	}

	req := newRequest(largePayload)
	req.Header.Set("X-Api-Key", "sk-test-123")
	w := httptest.NewRecorder()

	gw.HandleIngestEvent(w, req)

	// MaxBytesReader returns a 413 Payload Too Large error when the limit is exceeded
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
