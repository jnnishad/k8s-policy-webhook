package policy

import "testing"

func basePod() Pod {
	return Pod{
		Metadata: Metadata{
			Name:   "web-1",
			Labels: map[string]string{"team": "platform", "app": "web"},
		},
		Spec: PodSpec{
			Containers: []Container{
				{
					Name:  "web",
					Image: "registry.internal/web:1.4.2",
					Resources: Resources{
						Requests: map[string]string{"cpu": "100m", "memory": "128Mi"},
						Limits:   map[string]string{"cpu": "500m", "memory": "256Mi"},
					},
				},
			},
		},
	}
}

func TestEvaluate_CleanPodHasNoViolations(t *testing.T) {
	pod := basePod()
	got := Evaluate(pod)
	if len(got) != 0 {
		t.Fatalf("expected no violations, got %v", got)
	}
}

func TestEvaluate_MissingLabels(t *testing.T) {
	pod := basePod()
	pod.Metadata.Labels = map[string]string{"team": "platform"} // missing "app"

	got := Evaluate(pod)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %v", len(got), got)
	}
	if got[0].Rule != "required-labels" {
		t.Errorf("expected required-labels violation, got rule %q", got[0].Rule)
	}
}

func TestEvaluate_NoLabelsAtAll(t *testing.T) {
	pod := basePod()
	pod.Metadata.Labels = nil

	got := Evaluate(pod)
	if len(got) != len(RequiredLabels) {
		t.Fatalf("expected %d violations (one per required label), got %d: %v", len(RequiredLabels), len(got), got)
	}
}

func TestCheckImageTag(t *testing.T) {
	cases := []struct {
		name      string
		image     string
		wantViol  bool
		wantRule  string
	}{
		{"pinned version is fine", "nginx:1.25.3", false, ""},
		{"digest pin is fine", "nginx@sha256:abc123def456", false, ""},
		{"registry with port and pinned tag", "registry.internal:5000/nginx:1.25.3", false, ""},
		{"explicit latest is rejected", "nginx:latest", true, "no-floating-tags"},
		{"no tag at all is rejected", "nginx", true, "no-floating-tags"},
		{"registry with port but no tag is rejected", "registry.internal:5000/nginx", true, "no-floating-tags"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Container{Name: "c", Image: tc.image}
			got := checkImageTag(c)
			if tc.wantViol && len(got) == 0 {
				t.Fatalf("expected a violation for image %q, got none", tc.image)
			}
			if !tc.wantViol && len(got) != 0 {
				t.Fatalf("expected no violation for image %q, got %v", tc.image, got)
			}
			if tc.wantViol && got[0].Rule != tc.wantRule {
				t.Errorf("expected rule %q, got %q", tc.wantRule, got[0].Rule)
			}
		})
	}
}

func TestCheckResources_MissingRequestsAndLimits(t *testing.T) {
	c := Container{Name: "c", Image: "nginx:1.25.3"}
	got := checkResources(c)
	if len(got) != 2 {
		t.Fatalf("expected 2 violations (requests + limits), got %d: %v", len(got), got)
	}
}

func TestEvaluate_MultipleContainersAccumulateViolations(t *testing.T) {
	pod := basePod()
	pod.Spec.Containers = append(pod.Spec.Containers, Container{
		Name:  "sidecar",
		Image: "logging-agent:latest",
	})

	got := Evaluate(pod)
	// sidecar: latest tag + no requests + no limits = 3 violations
	if len(got) != 3 {
		t.Fatalf("expected 3 violations from the second container, got %d: %v", len(got), got)
	}
}
