// Package api exposes the HTTP (REST) handlers consumed by the DespHub backend.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/paulorosantos/desphub-rpa/internal/logger"
	"github.com/paulorosantos/desphub-rpa/internal/model"
	"github.com/paulorosantos/desphub-rpa/internal/query"
)

type Server struct {
	log      *slog.Logger
	svc      *query.Service 
	tokenKey string         
}

func NewServer(log *slog.Logger, svc *query.Service, tokenKey string) *Server {
	return &Server{log: log, svc: svc, tokenKey: tokenKey}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/detran/queries", s.handleQuery)
	mux.HandleFunc("POST /api/detran/session/reconnect", s.handleReconnect)
	mux.HandleFunc("POST /api/detran/session/token", s.handleSetToken)
	mux.HandleFunc("GET /api/detran/session", s.handleSessionStatus)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	jobID := uuid.NewString()
	ctx := logger.WithJobID(r.Context(), jobID)
	log := logger.FromContext(ctx, s.log)

	var req model.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("invalid payload", "error", err.Error())
		writeJSON(w, http.StatusBadRequest, model.QueryResponse{
			JobID:  jobID,
			Status: model.StatusError,
			Errors: []model.StepError{{Step: "decode", Message: "invalid JSON"}},
		})
		return
	}

	if errs := validate(req); len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, model.QueryResponse{
			JobID:  jobID,
			Plate:  req.Plate,
			Status: model.StatusError,
			Errors: errs,
		})
		return
	}

	log.Info("query received", "plate", maskPlate(req.Plate), "types", req.Types)

	if s.svc == nil {
		writeJSON(w, http.StatusNotImplemented, model.QueryResponse{
			JobID:       jobID,
			Plate:       req.Plate,
			Source:      "DETRAN-RS",
			CollectedAt: time.Now().UTC(),
			Status:      model.StatusError,
			Errors: []model.StepError{{
				Step:    "config",
				Message: "login not configured (set LOGIN_MODE=manual or token)",
			}},
		})
		return
	}

	resp := s.svc.Execute(ctx, jobID, req)
	status := http.StatusOK
	if resp.Status == model.StatusError {
		status = http.StatusBadGateway
		if hasStep(resp.Errors, "reconnect") {
			status = http.StatusServiceUnavailable
		}
	}
	writeJSON(w, status, resp)
}

func (s *Server) handleReconnect(w http.ResponseWriter, r *http.Request) {
	if s.svc == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"status": "error", "message": "login not configured (LOGIN_MODE=manual)",
		})
		return
	}
	s.log.Info("manual reconnect requested")
	if err := s.svc.Reconnect(r.Context()); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"status": "error", "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reconnected"})
}

func (s *Server) handleSetToken(w http.ResponseWriter, r *http.Request) {
	if s.svc == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"status": "error", "message": "portal not configured (LOGIN_MODE=token)",
		})
		return
	}
	if s.tokenKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Api-Key")), []byte(s.tokenKey)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"status": "error", "message": "invalid X-Api-Key",
		})
		return
	}

	var body struct {
		Bearer string `json:"bearer"`
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Bearer == "" || body.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "error", "message": "provide bearer and userId",
		})
		return
	}
	if err := s.svc.SetToken(body.Bearer, body.UserID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"status": "error", "message": err.Error(),
		})
		return
	}
	s.log.Info("session token updated via endpoint")
	writeJSON(w, http.StatusOK, map[string]string{"status": "token updated"})
}

func (s *Server) handleSessionStatus(w http.ResponseWriter, _ *http.Request) {
	if s.svc == nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "mode": "disabled"})
		return
	}
	authenticated, expiresAt, ok := s.svc.SessionStatus()
	resp := map[string]any{"authenticated": authenticated, "reportsStatus": ok}
	if !expiresAt.IsZero() {
		resp["expiresAt"] = expiresAt.UTC()
		resp["expiresInSeconds"] = int(time.Until(expiresAt).Seconds())
	}
	writeJSON(w, http.StatusOK, resp)
}

func hasStep(errs []model.StepError, step string) bool {
	for _, e := range errs {
		if e.Step == step {
			return true
		}
	}
	return false
}

func validate(req model.QueryRequest) []model.StepError {
	var errs []model.StepError
	if strings.TrimSpace(req.Plate) == "" && strings.TrimSpace(req.Renavam) == "" {
		errs = append(errs, model.StepError{
			Step:    "validation",
			Message: "provide at least plate or renavam",
		})
	}
	if len(req.Types) == 0 {
		errs = append(errs, model.StepError{
			Step:    "validation",
			Message: "provide at least one query type",
		})
	}
	return errs
}

func maskPlate(plate string) string {
	plate = strings.TrimSpace(plate)
	if len(plate) <= 3 {
		return plate
	}
	return plate[:3] + strings.Repeat("*", len(plate)-3)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
