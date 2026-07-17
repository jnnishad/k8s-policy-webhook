// Command webhook runs the k8s-policy-webhook HTTPS server. The
// Kubernetes API server requires admission webhooks to be served over
// TLS, so this expects a cert/key pair — in cluster, cert-manager
// mounts these via the Secret referenced in deploy/certificate.yaml.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/jnnishad/k8s-policy-webhook/internal/webhook"
)

func main() {
	addr := flag.String("addr", ":8443", "address to serve the webhook on")
	certFile := flag.String("tls-cert", "/etc/webhook/tls/tls.crt", "path to TLS certificate")
	keyFile := flag.String("tls-key", "/etc/webhook/tls/tls.key", "path to TLS private key")
	flag.Parse()

	logger := log.New(os.Stdout, "k8s-policy-webhook: ", log.LstdFlags)
	handler := webhook.NewHandler(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", handler.ServeHTTP)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	logger.Printf("listening on %s", *addr)
	if err := http.ListenAndServeTLS(*addr, *certFile, *keyFile, mux); err != nil {
		logger.Fatalf("server exited: %v", err)
	}
}
