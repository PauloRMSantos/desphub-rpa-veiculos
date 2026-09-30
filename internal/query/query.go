package query

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/logger"
	"github.com/paulorosantos/desphub-rpa/internal/model"
	"github.com/paulorosantos/desphub-rpa/internal/pii"
)

type Portal interface {
	Query(ctx context.Context, plate, renavam string, creds *model.SessionCredentials) (*model.QueryResponse, error)
	Name() string
}

type ParseFunc func(raw []byte) (*model.QueryResponse, error)

type Service struct {
	portal    Portal               
	parsers   map[string]ParseFunc 
	log       *slog.Logger
	anonymize bool
}

func NewService(portal Portal, log *slog.Logger, anonymize bool) *Service {
	return &Service{portal: portal, parsers: map[string]ParseFunc{}, log: log, anonymize: anonymize}
}

func (s *Service) RegisterParser(state string, fn ParseFunc) {
	s.parsers[strings.ToUpper(strings.TrimSpace(state))] = fn
}

func (s *Service) Execute(ctx context.Context, jobID string, req model.QueryRequest) model.QueryResponse {
	log := logger.FromContext(ctx, s.log)
	now := time.Now().UTC()
	state := strings.ToUpper(strings.TrimSpace(req.State))
	if state == "" {
		state = "RS"
	}

	fail := func(step, msg string) model.QueryResponse {
		return model.QueryResponse{
			JobID: jobID, Plate: req.Plate, CollectedAt: now,
			Status: model.StatusError,
			Errors: []model.StepError{{Step: step, Message: msg}},
		}
	}

	if fn, ok := s.parsers[state]; ok {
		if len(req.Payload) == 0 {
			return fail("payload", "state "+state+" requires a captured payload")
		}
		resp, err := fn(req.Payload)
		if err != nil {
			log.Error("normalize failed", "state", state, "error", err.Error())
			return fail("parse", err.Error())
		}
		return s.finalize(resp, jobID, req, now)
	}

	if s.portal == nil {
		return fail("config", "state "+state+" not configured (set LOGIN_MODE=token|manual)")
	}
	resp, err := s.portal.Query(ctx, req.Plate, req.Renavam, req.Session)
	if err != nil {
		step := "portal"
		if errors.Is(err, model.ErrReconnectRequired) {
			step = "reconnect"
		}
		log.Error("portal query failed", "step", step, "error", err.Error())
		return fail(step, err.Error())
	}
	if resp.Source == "" {
		resp.Source = s.portal.Name()
	}
	return s.finalize(resp, jobID, req, now)
}

func (s *Service) finalize(resp *model.QueryResponse, jobID string, req model.QueryRequest, now time.Time) model.QueryResponse {
	resp.JobID = jobID
	resp.CollectedAt = now
	if resp.Plate == "" {
		resp.Plate = req.Plate
	}
	resp.Status = classify(resp)
	if s.anonymize {
		pii.Anonymize(resp)
	}
	return *resp
}

type Reconnectable interface {
	Reconnect(ctx context.Context) error
}

func (s *Service) Reconnect(ctx context.Context) error {
	if r, ok := s.portal.(Reconnectable); ok {
		return r.Reconnect(ctx)
	}
	return errors.New("portal does not support manual reconnection")
}

type TokenSetter interface {
	SetToken(bearer, userID string)
}

type StatusProvider interface {
	Status() (authenticated bool, expiresAt time.Time)
}

func (s *Service) SetToken(bearer, userID string) error {
	if ts, ok := s.portal.(TokenSetter); ok {
		ts.SetToken(bearer, userID)
		return nil
	}
	return errors.New("portal does not accept token injection (use LOGIN_MODE=token)")
}

func (s *Service) SessionStatus() (authenticated bool, expiresAt time.Time, ok bool) {
	if sp, ok2 := s.portal.(StatusProvider); ok2 {
		a, e := sp.Status()
		return a, e, true
	}
	return false, time.Time{}, false
}

func classify(r *model.QueryResponse) model.Status {
	if r.Vehicle == nil || r.Vehicle.MakeModel == "" {
		return model.StatusPartial
	}
	return model.StatusSuccess
}
