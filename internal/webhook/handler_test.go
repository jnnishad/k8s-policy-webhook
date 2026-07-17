package webhook

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler() *Handler {
	return NewHandler(log.New(bytesDiscard{}, "", 0))
}

type bytesDiscard struct{}

func (bytesDiscard) Write(p []byte) (int, error) { return len(p), nil }

func doRequest(t *testing.T, h *Handler, review AdmissionReview) AdmissionReview {
	t.Helper()
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var out AdmissionReview
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return out
}

func TestServeHTTP_AllowsCompliantPod(t *testing.T) {
	h := newTestHandler()

	podJSON := `{
		"metadata": {"name": "web-1", "labels": {"team": "platform", "app": "web"}},
		"spec": {"containers": [{
			"name": "web",
			"image": "registry.internal/web:1.4.2",
			"resources": {"requests": {"cpu": "100m"}, "limits": {"cpu": "500m"}}
		}]}
	}`

	review := AdmissionReview{
		APIVersion: "admission.k8s.io/v1",
		Kind:       "AdmissionReview",
		Request: &AdmissionRequest{
			UID:    "abc-123",
			Object: json.RawMessage(podJSON),
		},
	}

	out := doRequest(t, h, review)

	if out.Response == nil {
		t.Fatal("expected a response, got nil")
	}
	if out.Response.UID != "abc-123" {
		t.Errorf("expected UID to be echoed back, got %q", out.Response.UID)
	}
	if !out.Response.Allowed {
		t.Errorf("expected compliant pod to be allowed, got denied: %+v", out.Response.Status)
	}
}

func TestServeHTTP_RejectsNonCompliantPod(t *testing.T) {
	h := newTestHandler()

	podJSON := `{
		"metadata": {"name": "web-1", "labels": {}},
		"spec": {"containers": [{"name": "web", "image": "web:latest"}]}
	}`

	review := AdmissionReview{
		Request: &AdmissionRequest{
			UID:    "def-456",
			Object: json.RawMessage(podJSON),
		},
	}

	out := doRequest(t, h, review)

	if out.Response.Allowed {
		t.Fatal("expected non-compliant pod to be rejected")
	}
	if out.Response.Status == nil || out.Response.Status.Message == "" {
		t.Fatal("expected a rejection message explaining why")
	}
	// Should mention the missing labels and the floating tag.
	msg := out.Response.Status.Message
	for _, want := range []string{"team", "app", "latest"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected rejection message to mention %q, got: %s", want, msg)
		}
	}
}

func TestServeHTTP_MalformedBodyReturns400(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for malformed body, got %d", rec.Code)
	}
}

func TestServeHTTP_MissingRequestReturns400(t *testing.T) {
	h := newTestHandler()

	review := AdmissionReview{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"}
	body, _ := json.Marshal(review)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 when request field is nil, got %d", rec.Code)
	}
}
