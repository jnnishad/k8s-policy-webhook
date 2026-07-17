// Package policy contains the pure, dependency-free rules the webhook
// evaluates against a Pod spec. Kept separate from the HTTP/admission
// plumbing so every rule can be unit tested without a fake API server.
package policy

import (
	"fmt"
	"strings"
)

// Pod is a minimal, hand-rolled mirror of the fields of corev1.Pod this
// package actually needs. Using our own struct instead of importing
// k8s.io/api keeps the module dependency-free: it's just JSON in, JSON
// out, which is all an admission webhook ever really is.
type Pod struct {
	Metadata Metadata `json:"metadata"`
	Spec     PodSpec  `json:"spec"`
}

type Metadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

type PodSpec struct {
	Containers []Container `json:"containers"`
}

type Container struct {
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Resources Resources `json:"resources"`
}

type Resources struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// RequiredLabels are the labels every Pod must carry so it's traceable
// back to an owning team and application during an incident.
var RequiredLabels = []string{"team", "app"}

// Violation describes a single failed rule, with enough detail that the
// rejection message a developer sees in `kubectl apply` output tells
// them exactly what to fix.
type Violation struct {
	Rule    string
	Message string
}

func (v Violation) String() string {
	return fmt.Sprintf("[%s] %s", v.Rule, v.Message)
}

// Evaluate runs every rule against pod and returns all violations found
// (not just the first one), so a developer can fix everything in one
// pass instead of playing whack-a-mole with repeated `kubectl apply`.
func Evaluate(pod Pod) []Violation {
	var violations []Violation

	violations = append(violations, checkRequiredLabels(pod)...)

	for _, c := range pod.Spec.Containers {
		violations = append(violations, checkImageTag(c)...)
		violations = append(violations, checkResources(c)...)
	}

	return violations
}

func checkRequiredLabels(pod Pod) []Violation {
	var violations []Violation
	for _, key := range RequiredLabels {
		if _, ok := pod.Metadata.Labels[key]; !ok {
			violations = append(violations, Violation{
				Rule:    "required-labels",
				Message: fmt.Sprintf("pod %q is missing required label %q", pod.Metadata.Name, key),
			})
		}
	}
	return violations
}

// checkImageTag rejects images with no tag (defaults to :latest) or an
// explicit :latest tag. Floating tags make rollbacks non-deterministic:
// a redeployed manifest can silently pull a different image than the
// one that was originally running.
func checkImageTag(c Container) []Violation {
	image := c.Image
	// A digest pin (image@sha256:...) is always acceptable.
	if strings.Contains(image, "@sha256:") {
		return nil
	}

	lastColon := strings.LastIndex(image, ":")
	lastSlash := strings.LastIndex(image, "/")
	hasTag := lastColon > lastSlash

	if !hasTag {
		return []Violation{{
			Rule:    "no-floating-tags",
			Message: fmt.Sprintf("container %q image %q has no tag; it will resolve to :latest", c.Name, image),
		}}
	}

	tag := image[lastColon+1:]
	if tag == "latest" {
		return []Violation{{
			Rule:    "no-floating-tags",
			Message: fmt.Sprintf("container %q image %q is pinned to :latest; use an explicit version or digest", c.Name, image),
		}}
	}

	return nil
}

// checkResources requires both requests and limits on every container.
// Without requests, the scheduler can't bin-pack correctly. Without
// limits, one noisy container can starve its neighbors on a shared node.
func checkResources(c Container) []Violation {
	var violations []Violation
	if len(c.Resources.Requests) == 0 {
		violations = append(violations, Violation{
			Rule:    "require-resource-requests",
			Message: fmt.Sprintf("container %q has no resource requests set", c.Name),
		})
	}
	if len(c.Resources.Limits) == 0 {
		violations = append(violations, Violation{
			Rule:    "require-resource-limits",
			Message: fmt.Sprintf("container %q has no resource limits set", c.Name),
		})
	}
	return violations
}
