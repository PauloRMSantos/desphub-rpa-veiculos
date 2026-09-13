package pii

import (
	"strings"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

func Anonymize(r *model.QueryResponse) {
	if r == nil || r.Vehicle == nil {
		return
	}
	r.Vehicle.OwnerCPF = MaskCPF(r.Vehicle.OwnerCPF)
}

func MaskCPF(cpf string) string {
	d := onlyDigits(cpf)
	switch {
	case d == "":
		return ""
	case len(d) == 11:
		return "***." + d[3:6] + "." + d[6:9] + "-**"
	case len(d) <= 2:
		return strings.Repeat("*", len(d))
	default:
		return strings.Repeat("*", len(d)-2) + d[len(d)-2:]
	}
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
