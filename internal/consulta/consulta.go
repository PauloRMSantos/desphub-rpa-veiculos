package consulta

import (
	"context"
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
		log.Error("falha na consulta ao portal", "etapa", "portal", "erro", err.Error())
		base.Status = model.StatusErro
		base.Erros = []model.EtapaErro{{Etapa: "portal", Mensagem: err.Error()}}
		return base
	}

	// Preserva os metadados do orquestrador (jobId/timestamp) sobre o parse.
	resp.JobID = jobID
	resp.ColetadoEm = base.ColetadoEm
	if resp.Fonte == "" {
		resp.Fonte = s.portal.Nome()
	}
	if resp.Placa == "" {
		resp.Placa = req.Placa
	}
	resp.Status = classificar(resp)

	// Política de saída: anonimiza PII quando ligado (LGPD).
	if s.anonimizar {
		pii.Anonimizar(resp)
	}
	return *resp
}

// classificar decide SUCESSO/PARCIAL com base no que foi extraído.
func classificar(r *model.ConsultaResponse) model.Status {
	if r.Veiculo == nil || r.Veiculo.MarcaModelo == "" {
		return model.StatusParcial
	}
	return model.StatusSucesso
}
