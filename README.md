# k8s-policy-webhook

A dependency-free Go **ValidatingAdmissionWebhook** for Kubernetes: it
rejects Pods at `kubectl apply` time — before they ever hit the
scheduler — if they're missing ownership labels, running on a floating
`:latest` tag, or shipped without resource requests/limits.

## Why

[`ansible-infra-automation`](https://github.com/jnnishad/ansible-infra-automation)
covers hardening at the *node* level; this repo covers the same
intent — "don't let an unsafe config reach production" — at the
*workload* level, enforced by the API server itself instead of a
linter someone forgot to run. It's the same standards used to keep
production Kubernetes on-prem (OpenStack) and in the cloud (AWS)
consistent, just expressed as code the cluster enforces automatically
rather than a checklist.

## What it enforces

| Rule | Why it matters |
|---|---|
| `required-labels` | Every Pod needs `team` and `app` labels — otherwise "whose pod is eating this node's CPU at 2am" is a 20-minute investigation instead of a `kubectl get pod -o yaml`. |
| `no-floating-tags` | `:latest` (or no tag at all) means a redeployed manifest can silently pull a different image than the one that was actually tested. Digest pins (`@sha256:...`) are always accepted. |
| `require-resource-requests` / `require-resource-limits` | No requests breaks scheduler bin-packing; no limits lets one noisy container starve its neighbors on a shared node. |

## Why hand-rolled admission types instead of `k8s.io/api`

The `AdmissionReview` wire format is small, stable, and just JSON. This
module intentionally has **zero external dependencies** — no
`k8s.io/api`, no `client-go` — so `go build` doesn't need to pull half
of Kubernetes' module graph to compile a ~200-line webhook, and every
byte the API server sends is visible in `internal/webhook/admission.go`
rather than several layers deep in a vendored SDK.

## Structure

```
cmd/webhook/            entrypoint — flags, TLS server, routing
internal/policy/         the actual rules (policy.go), pure functions, no HTTP/K8s types
internal/webhook/        AdmissionReview types + HTTP handler that glues policy -> response
deploy/                  Deployment, Service, cert-manager Certificate, ValidatingWebhookConfiguration
```

`internal/policy` has no knowledge of HTTP or the admission API at
all — it takes a `Pod` struct and returns violations. That's what makes
`policy_test.go` fast and trivial: no fake API server, no `httptest`,
just table-driven tests against plain structs.

## Run it

```bash
make test              # go test ./... -race -cover
make vet
make build              # -> bin/webhook

# local run against a self-signed dev cert
mkdir -p dev-certs && openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout dev-certs/tls.key -out dev-certs/tls.crt -days 365 -subj "/CN=localhost"
make run
```

## Deploy

```bash
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/certificate.yaml               # requires cert-manager
kubectl apply -f deploy/validatingwebhookconfiguration.yaml
```

Ships with `failurePolicy: Ignore` — safe to roll out cold. Once you've
watched rejections in the logs for a day or two with nothing on fire,
flip it to `Fail`.

## Status

Built to demonstrate the pattern end to end (policy engine, admission
plumbing, deploy manifests, CI). Not running against a live cluster
yet — the natural next steps are wiring in `namespaceSelector` exemptions
per-team and moving from `Ignore` to `Fail` once it's been observed in
dry-run for a rollout window.

I did not have a Go toolchain available in the environment this was
written in, so `go build`/`go test` have not actually been executed
against this code — every test case was hand-traced instead of run.
Run `make test` before treating this as verified; if anything fails,
it's a fast fix, not a redesign.

<!-- test commit 2026-02-23T00:48:59 -->

<!-- test commit 2026-06-14T20:34:08 -->

<!-- test commit 2026-02-13T08:13:43 -->

<!-- test commit 2026-02-28T13:57:31 -->

<!-- JN -->

<!-- JN -->

<!-- JN -->

<!-- JN -->

<!-- JN -->
