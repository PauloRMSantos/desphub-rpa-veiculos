package consulta

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
	Consultar(ctx context.Context, placa, renavam string) (*model.ConsultaResponse, error)
	Nome() string
}

type Service struct {
	portal     Portal
	log        *slog.Logger
	anonimizar bool // mascara PII na saída (config ANONIMIZAR_PII)
}

func NewService(portal Portal, log *slog.Logger, anonimizar bool) *Service {
	return &Service{portal: portal, log: log, anonimizar: anonimizar}
}

func (s *Service) Executar(ctx context.Context, jobID string, req model.ConsultaRequest) model.ConsultaResponse {
	log := logger.FromContext(ctx, s.log)
	base := model.ConsultaResponse{
		JobID:      jobID,
		Placa:      req.Placa,
		Fonte:      s.portal.Nome(),
		ColetadoEm: time.Now().UTC(),
	}

	resp, err := s.portal.Consultar(ctx, req.Placa, req.Renavam)
	if err != nil {
		etapa := "portal"
		if errors.Is(err, model.ErrReconexaoNecessaria) {
			etapa = "reconexao"
		}
		log.Error("falha na consulta ao portal", "etapa", etapa, "erro", err.Error())
		base.Status = model.StatusErro
		base.Erros = []model.EtapaErro{{Etapa: etapa, Mensagem: err.Error()}}
		return base
	}

	resp.JobID = jobID
	resp.ColetadoEm = base.ColetadoEm
	if resp.Fonte == "" {
		resp.Fonte = s.portal.Nome()
	}
	if resp.Placa == "" {
		resp.Placa = req.Placa
	}
	resp.Status = classificar(resp)

	if s.anonimizar {
		pii.Anonimizar(resp)
	}
	return *resp
}

type Reconectavel interface {
	Reconectar(ctx context.Context) error
}

func (s *Service) Reconectar(ctx context.Context) error {
	if r, ok := s.portal.(Reconectavel); ok {
		return r.Reconectar(ctx)
	}
	return errors.New("portal não suporta reconexão manual")
}

type TokenSetter interface {
	SetToken(bearer, userID string)
}

type StatusProvider interface {
	Status() (autenticado bool, expiraEm time.Time)
}

func (s *Service) DefinirToken(bearer, userID string) error {
	if ts, ok := s.portal.(TokenSetter); ok {
		ts.SetToken(bearer, userID)
		return nil
	}
	return errors.New("portal não aceita injeção de token (use LOGIN_MODE=token)")
}

func (s *Service) StatusSessao() (autenticado bool, expiraEm time.Time, ok bool) {
	if sp, ok2 := s.portal.(StatusProvider); ok2 {
		a, e := sp.Status()
		return a, e, true
	}
	return false, time.Time{}, false
}

func classificar(r *model.ConsultaResponse) model.Status {
	if r.Veiculo == nil || r.Veiculo.MarcaModelo == "" {
		return model.StatusParcial
	}
	return model.StatusSucesso
}
