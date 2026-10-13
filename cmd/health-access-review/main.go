package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/health-access-review/internal/infrai"
	"example.com/health-access-review/internal/review"
)

type server struct {
	client *infrai.Client
	now    func() time.Time
}

type reviewRequest struct {
	Appointments []review.AppointmentChange `json:"appointments"`
}

type reviewResponse struct {
	Report   review.Report   `json:"report"`
	Document json.RawMessage `json:"document"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	s := &server{client: infrai.New(key), now: time.Now}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /reviews", s.createReview)
	log.Printf("health access review listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) createReview(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input reviewRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid review request"})
		return
	}

	keys, err := s.client.ListKeys(r.Context())
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	report := review.Build(keys, input.Appointments, s.now())
	document, err := s.client.GeneratePDF(r.Context(), infrai.GeneratePDFRequest{
		Markdown:    review.Markdown(report),
		PageSize:    "A4",
		Orientation: "portrait",
		Store:       false,
	})
	if err != nil {
		writeUpstreamError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, reviewResponse{Report: report, Document: document})
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		writeJSON(w, apiErr.Status, map[string]string{"error": apiErr.Code, "message": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
