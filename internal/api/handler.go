package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/paulorosantos/desphub-rpa/internal/consulta"
	"github.com/paulorosantos/desphub-rpa/internal/logger"
	"github.com/paulorosantos/desphub-rpa/internal/model"
)

type Server struct {
	log *slog.Logger
	svc *consulta.Service
}

func NewServer(log *slog.Logger, svc *consulta.Service) *Server {
	return &Server{log: log, svc: svc}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/consultas", s.handleConsulta)
	mux.HandleFunc("POST /api/v1/sessao/reconectar", s.handleReconectar)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleConsulta(w http.ResponseWriter, r *http.Request) {
	jobID := uuid.NewString()
	ctx := logger.WithJobID(r.Context(), jobID)
	log := logger.FromContext(ctx, s.log)

	var req model.ConsultaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("payload inválido", "erro", err.Error())
		writeJSON(w, http.StatusBadRequest, model.ConsultaResponse{
			JobID:  jobID,
			Status: model.StatusErro,
			Erros:  []model.EtapaErro{{Etapa: "decode", Mensagem: "JSON inválido"}},
		})
		return
	}

	if errs := validar(req); len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, model.ConsultaResponse{
			JobID:  jobID,
			Placa:  req.Placa,
			Status: model.StatusErro,
			Erros:  errs,
		})
		return
	}

	log.Info("consulta recebida",
		"placa", mascararPlaca(req.Placa),
		"tipos", req.Tipos,
	)

	if s.svc == nil {
		writeJSON(w, http.StatusNotImplemented, model.ConsultaResponse{
			JobID:      jobID,
			Placa:      req.Placa,
			Fonte:      "DETRAN-RS",
			ColetadoEm: time.Now().UTC(),
			Status:     model.StatusErro,
			Erros: []model.EtapaErro{{
				Etapa:    "config",
				Mensagem: "login não configurado (defina LOGIN_MODE=manual)",
			}},
		})
		return
	}

	resp := s.svc.Executar(ctx, jobID, req)
	status := http.StatusOK
	if resp.Status == model.StatusErro {
		status = http.StatusBadGateway // falha ao falar com o portal
		if temEtapa(resp.Erros, "reconexao") {
			// Sessão gov.br expirou: 503 sinaliza "indisponível até reconectar".
			status = http.StatusServiceUnavailable
		}
	}
	writeJSON(w, status, resp)
}

func (s *Server) handleReconectar(w http.ResponseWriter, r *http.Request) {
	if s.svc == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"status": "erro", "mensagem": "login não configurado (LOGIN_MODE=manual)",
		})
		return
	}
	s.log.Info("reconexão manual solicitada")
	if err := s.svc.Reconectar(r.Context()); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"status": "erro", "mensagem": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reconectado"})
}

func temEtapa(erros []model.EtapaErro, etapa string) bool {
	for _, e := range erros {
		if e.Etapa == etapa {
			return true
		}
	}
	return false
}

func validar(req model.ConsultaRequest) []model.EtapaErro {
	var errs []model.EtapaErro
	if strings.TrimSpace(req.Placa) == "" && strings.TrimSpace(req.Renavam) == "" {
		errs = append(errs, model.EtapaErro{
			Etapa:    "validacao",
			Mensagem: "informe ao menos placa ou renavam",
		})
	}
	if len(req.Tipos) == 0 {
		errs = append(errs, model.EtapaErro{
			Etapa:    "validacao",
			Mensagem: "informe ao menos um tipo de consulta",
		})
	}
	return errs
}

func mascararPlaca(placa string) string {
	placa = strings.TrimSpace(placa)
	if len(placa) <= 3 {
		return placa
	}
	return placa[:3] + strings.Repeat("*", len(placa)-3)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
