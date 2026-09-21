package query

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/logger"
	"github.com/paulorosantos/desphub-rpa/internal/model"
	"github.com/paulorosantos/desphub-rpa/internal/pii"
)

type Portal interface {
	// Query runs a vehicle query. creds carries per-request (multi-tenant)
	// session credentials; when nil the portal uses its own session/token.
	Query(ctx context.Context, plate, renavam string, creds *model.SessionCredentials) (*model.QueryResponse, error)
	Name() string
}

type Service struct {
	portal    Portal
	log       *slog.Logger
	anonymize bool 
}

func NewService(portal Portal, log *slog.Logger, anonymize bool) *Service {
	return &Service{portal: portal, log: log, anonymize: anonymize}
}

func (s *Service) Execute(ctx context.Context, jobID string, req model.QueryRequest) model.QueryResponse {
	log := logger.FromContext(ctx, s.log)
	base := model.QueryResponse{
		JobID:       jobID,
		Plate:       req.Plate,
		Source:      s.portal.Name(),
		CollectedAt: time.Now().UTC(),
	}

	resp, err := s.portal.Query(ctx, req.Plate, req.Renavam, req.Session)
	if err != nil {
		step := "portal"
		if errors.Is(err, model.ErrReconnectRequired) {
			step = "reconnect"
		}
		log.Error("portal query failed", "step", step, "error", err.Error())
		base.Status = model.StatusError
		base.Errors = []model.StepError{{Step: step, Message: err.Error()}}
		return base
	}

	resp.JobID = jobID
	resp.CollectedAt = base.CollectedAt
	if resp.Source == "" {
		resp.Source = s.portal.Name()
	}
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
