package health

import (
	"encoding/json"
	"net/http"
)

// NewServer returns an HTTP server that responds to GET /healthz with {"status":"ok"}.
func NewServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	return &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
}
