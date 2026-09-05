package consulta

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

// portalFake implementa Portal sem tocar rede.
type portalFake struct {
	resp *model.ConsultaResponse
	err  error
}

func (p portalFake) Nome() string { return "FAKE" }
func (p portalFake) Consultar(_ context.Context, _, _ string) (*model.ConsultaResponse, error) {
	return p.resp, p.err
}

func newLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// portalReconectavel implementa Portal + Reconectavel.
type portalReconectavel struct {
	portalFake
	reconectouChamado bool
	reconectarErr     error
}

func (p *portalReconectavel) Reconectar(_ context.Context) error {
	p.reconectouChamado = true
	return p.reconectarErr
}

func TestExecutarReconexaoNecessariaMarcaEtapa(t *testing.T) {
	fake := portalFake{err: model.ErrReconexaoNecessaria}
	svc := NewService(fake, newLog(), false)

	got := svc.Executar(context.Background(), "job-r", model.ConsultaRequest{Placa: "ABC1D23"})

	if got.Status != model.StatusErro {
		t.Fatalf("status = %q; quero ERRO", got.Status)
	}
	if len(got.Erros) != 1 || got.Erros[0].Etapa != "reconexao" {
		t.Errorf("esperava etapa 'reconexao', veio %+v", got.Erros)
	}
}

func TestReconectarChamaPortal(t *testing.T) {
	fake := &portalReconectavel{}
	svc := NewService(fake, newLog(), false)

	if err := svc.Reconectar(context.Background()); err != nil {
		t.Fatalf("Reconectar erro: %v", err)
	}
	if !fake.reconectouChamado {
		t.Error("esperava que o portal.Reconectar fosse chamado")
	}
}

func TestReconectarPortalSemSuporte(t *testing.T) {
	// portalFake não implementa Reconectavel.
	svc := NewService(portalFake{}, newLog(), false)
	if err := svc.Reconectar(context.Background()); err == nil {
		t.Error("esperava erro quando o portal não suporta reconexão")
	}
}

func TestExecutarSucesso(t *testing.T) {
	fake := portalFake{resp: &model.ConsultaResponse{
		Veiculo: &model.Veiculo{MarcaModelo: "VW/FUSCA"},
	}}
	svc := NewService(fake, newLog(), false)

	got := svc.Executar(context.Background(), "job-1", model.ConsultaRequest{Placa: "ABC1D23"})

	if got.Status != model.StatusSucesso {
		t.Errorf("status = %q; quero SUCESSO", got.Status)
	}
	if got.JobID != "job-1" {
		t.Errorf("jobId = %q; quero job-1", got.JobID)
	}
	if got.Fonte != "FAKE" {
		t.Errorf("fonte = %q; quero FAKE", got.Fonte)
	}
	if got.ColetadoEm.IsZero() {
		t.Error("coletadoEm não deveria ser zero")
	}
}

func TestExecutarErroViraStatusErro(t *testing.T) {
	fake := portalFake{err: errors.New("boom")}
	svc := NewService(fake, newLog(), false)

	got := svc.Executar(context.Background(), "job-2", model.ConsultaRequest{Placa: "ABC1D23"})

	if got.Status != model.StatusErro {
		t.Fatalf("status = %q; quero ERRO", got.Status)
	}
	if len(got.Erros) != 1 || got.Erros[0].Etapa != "portal" {
		t.Errorf("esperava 1 erro na etapa 'portal', veio %+v", got.Erros)
	}
}

func TestExecutarSemVeiculoViraParcial(t *testing.T) {
	fake := portalFake{resp: &model.ConsultaResponse{}}
	svc := NewService(fake, newLog(), false)

	got := svc.Executar(context.Background(), "job-3", model.ConsultaRequest{Placa: "ABC1D23"})

	if got.Status != model.StatusParcial {
		t.Errorf("status = %q; quero PARCIAL", got.Status)
	}
}

func TestAnonimizacaoLigadaMascaraCPF(t *testing.T) {
	fake := portalFake{resp: &model.ConsultaResponse{
		Veiculo: &model.Veiculo{MarcaModelo: "VW/FUSCA", CpfProprietario: "74722310025"},
	}}
	svc := NewService(fake, newLog(), true) // anonimização LIGADA

	got := svc.Executar(context.Background(), "job-4", model.ConsultaRequest{Placa: "ABC1D23"})

	if got.Veiculo.CpfProprietario != "***.223.100-**" {
		t.Errorf("CPF = %q; quero mascarado ***.223.100-**", got.Veiculo.CpfProprietario)
	}
}

func TestAnonimizacaoDesligadaPreservaCPF(t *testing.T) {
	fake := portalFake{resp: &model.ConsultaResponse{
		Veiculo: &model.Veiculo{MarcaModelo: "VW/FUSCA", CpfProprietario: "74722310025"},
	}}
	svc := NewService(fake, newLog(), false) // modo comercial/produção

	got := svc.Executar(context.Background(), "job-5", model.ConsultaRequest{Placa: "ABC1D23"})

	if got.Veiculo.CpfProprietario != "74722310025" {
		t.Errorf("CPF = %q; quero preservado (sem máscara)", got.Veiculo.CpfProprietario)
	}
}
