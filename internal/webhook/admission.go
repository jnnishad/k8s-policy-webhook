// Package webhook implements a Kubernetes ValidatingAdmissionWebhook
// HTTP handler. It speaks the admission.k8s.io/v1 AdmissionReview wire
// format directly, using hand-rolled structs instead of importing
// k8s.io/api — the format is stable, narrow, and just JSON, so this
// keeps the module dependency-free and easy to audit end to end.
package webhook

import "encoding/json"

// AdmissionReview is the top-level object the API server POSTs to the
// webhook, and the shape the webhook must echo back with a Response set.
type AdmissionReview struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Request    *AdmissionRequest  `json:"request,omitempty"`
	Response   *AdmissionResponse `json:"response,omitempty"`
}

type AdmissionRequest struct {
	UID       string          `json:"uid"`
	Namespace string          `json:"namespace"`
	Operation string          `json:"operation"`
	Object    json.RawMessage `json:"object"`
}

type AdmissionResponse struct {
	UID     string  `json:"uid"`
	Allowed bool    `json:"allowed"`
	Status  *Status `json:"status,omitempty"`
}

type Status struct {
	Message string `json:"message,omitempty"`
	Code    int32  `json:"code,omitempty"`
}
