package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gitlab.uis.dev/service/gocron/internal/models"
	"gitlab.uis.dev/service/gocron/internal/scheduler"
)

// Server is the HTTP server for the cron service.
type Server struct {
	log       *slog.Logger
	router    *chi.Mux
	scheduler scheduler.Interface
}

// NewServer creates a new HTTP server.
func NewServer(log *slog.Logger, schedulerInstance scheduler.Interface) *Server {
	s := &Server{
		log:       log,
		router:    chi.NewRouter(),
		scheduler: schedulerInstance,
	}

	s.setupMiddleware()
	s.registerRoutes()

	return s
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RequestID)
}

func (s *Server) registerRoutes() {
	s.router.Get("/", s.handleHealthCheck)
	s.router.Post("/jobs", s.handleCreateJob)
	// More routes to be added here
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req models.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Here you would typically use a validator like go-playground/validator
	// to validate the request struct.

	job, err := s.scheduler.CreateJob(r.Context(), req)
	if err != nil {
		s.log.Error("failed to create job", "error", err)
		// This could be a user error (e.g., duplicate custom_id) or a server error.
		respondWithError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}

	respondWithJSON(w, http.StatusCreated, job)
}

// respondWithJSON writes a JSON response.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// respondWithError writes an error JSON response.
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}
