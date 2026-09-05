// Package pii aplica anonimização de dados pessoais (LGPD) na saída do serviço.
//
// É uma política de SAÍDA: a coleta interna usa o dado real; a anonimização
// só decide o que sai na resposta. Controlada por config (ANONIMIZAR_PII).
//
// Nota legal: os dados veiculares são consultáveis publicamente no portal
// oficial do DETRAN-RS mediante placa/RENAVAM. A anonimização aqui é higiene
// de minimização de dados, não requisito de legalidade da consulta.
package pii

import (
	"strings"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

// Anonimizar mascara os campos pessoais sensíveis do response, in-place.
// Seguro para nil. Adicione novos campos aqui conforme o contrato cresce.
func Anonimizar(r *model.ConsultaResponse) {
	if r == nil || r.Veiculo == nil {
		return
	}
	r.Veiculo.CpfProprietario = MaskCPF(r.Veiculo.CpfProprietario)
}

// MaskCPF esconde os 3 primeiros e os 2 últimos dígitos, no estilo público
// de recibo: "74722310025" -> "***.223.100-**". Aceita CPF já formatado.
// Entrada vazia devolve vazio.
func MaskCPF(cpf string) string {
	d := somenteDigitos(cpf)
	switch {
	case d == "":
		return ""
	case len(d) == 11:
		return "***." + d[3:6] + "." + d[6:9] + "-**"
	case len(d) <= 2:
		return strings.Repeat("*", len(d))
	default:
		// Formato inesperado: revela no máximo os 2 últimos dígitos.
		return strings.Repeat("*", len(d)-2) + d[len(d)-2:]
	}
}

func somenteDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
