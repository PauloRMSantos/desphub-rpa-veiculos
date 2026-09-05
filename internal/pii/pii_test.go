package pii

import (
	"testing"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

func TestMaskCPF(t *testing.T) {
	casos := map[string]string{
		"74722310025":    "***.223.100-**",
		"747.223.100-25": "***.223.100-**", // já formatado
		"":               "",
		"12":             "**",
		"123456":         "****56", // formato inesperado: só 2 últimos
	}
	for in, want := range casos {
		if got := MaskCPF(in); got != want {
			t.Errorf("MaskCPF(%q) = %q; quero %q", in, got, want)
		}
	}
}

func TestAnonimizarSeguroParaNil(t *testing.T) {
	Anonimizar(nil)                                  // não deve entrar em pânico
	Anonimizar(&model.ConsultaResponse{})            // Veiculo nil, ok
	r := &model.ConsultaResponse{Veiculo: &model.Veiculo{CpfProprietario: "74722310025"}}
	Anonimizar(r)
	if r.Veiculo.CpfProprietario != "***.223.100-**" {
		t.Errorf("CPF = %q; quero mascarado", r.Veiculo.CpfProprietario)
	}
}
