package webhook

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/jnnishad/k8s-policy-webhook/internal/policy"
)

// Handler validates incoming Pod admission requests against the policy
// package's rules and returns an allow/deny AdmissionReview response.
type Handler struct {
	Logger *log.Logger
}

func NewHandler(logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{Logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var review AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		http.Error(w, "malformed AdmissionReview: "+err.Error(), http.StatusBadRequest)
		return
	}
	if review.Request == nil {
		http.Error(w, "AdmissionReview.request is nil", http.StatusBadRequest)
		return
	}

	response := h.evaluate(review.Request)

	out := AdmissionReview{
		APIVersion: "admission.k8s.io/v1",
		Kind:       "AdmissionReview",
		Response:   response,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		h.Logger.Printf("failed to encode response: %v", err)
	}
}

func (h *Handler) evaluate(req *AdmissionRequest) *AdmissionResponse {
	var pod policy.Pod
	if err := json.Unmarshal(req.Object, &pod); err != nil {
		return &AdmissionResponse{
			UID:     req.UID,
			Allowed: false,
			Status:  &Status{Message: "failed to decode pod: " + err.Error(), Code: http.StatusBadRequest},
		}
	}

	violations := policy.Evaluate(pod)
	if len(violations) == 0 {
		return &AdmissionResponse{UID: req.UID, Allowed: true}
	}

	messages := make([]string, len(violations))
	for i, v := range violations {
		messages[i] = v.String()
	}

	h.Logger.Printf("rejected pod %s/%s: %s", req.Namespace, podNameOrUnknown(pod), strings.Join(messages, "; "))

	return &AdmissionResponse{
		UID:     req.UID,
		Allowed: false,
		Status: &Status{
			Message: strings.Join(messages, "; "),
			Code:    http.StatusForbidden,
		},
	}
}

func podNameOrUnknown(pod policy.Pod) string {
	if pod.Metadata.Name != "" {
		return pod.Metadata.Name
	}
	return "(generateName, not yet assigned)"
}
