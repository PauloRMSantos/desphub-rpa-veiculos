package pii

import (
	"testing"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

func TestMaskCPF(t *testing.T) {
	cases := map[string]string{
		"74722310025":    "***.223.100-**",
		"747.223.100-25": "***.223.100-**", // already formatted
		"":               "",
		"12":             "**",
		"123456":         "****56", // unexpected format: last 2 only
	}
	for in, want := range cases {
		if got := MaskCPF(in); got != want {
			t.Errorf("MaskCPF(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestAnonymizeNilSafe(t *testing.T) {
	Anonymize(nil)                          // must not panic
	Anonymize(&model.QueryResponse{})       // Vehicle nil, ok
	r := &model.QueryResponse{Vehicle: &model.Vehicle{OwnerCPF: "74722310025"}}
	Anonymize(r)
	if r.Vehicle.OwnerCPF != "***.223.100-**" {
		t.Errorf("CPF = %q; want masked", r.Vehicle.OwnerCPF)
	}
}
