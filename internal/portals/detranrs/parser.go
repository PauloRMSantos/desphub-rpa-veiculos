package detranrs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

type apiResponse struct {
	Identificacao struct {
		TemErro             bool   `json:"temErro"`
		MarcaModelo         string `json:"marcaModelo"`
		Renavam             int64  `json:"renavam"`
		UfPlaca             string `json:"ufPlaca"`
		MunicipioRegistro   string `json:"municipioRegistro"`
		Tipo                string `json:"tipo"`
		Cor                 string `json:"cor"`
		Especie             string `json:"especie"`
		Placa               string `json:"placa"`
		Chassi              string `json:"chassi"`
		Situacao            string `json:"situacao"`
		CpfProprietario     string `json:"cpfProprietario"`
		Combustivel         string `json:"combustivel"`
		Categoria           string `json:"categoria"`
		AnoFabricacao       int    `json:"anoFabricacao"`
		AnoModelo           int    `json:"anoModelo"`
		DtVencLicenciamento string `json:"dtVencLicenciamento"`
		Furtado             bool   `json:"furtado"`
		EmDeposito          bool   `json:"emDeposito"`
	} `json:"identificacao"`

	Licenciamento struct {
		TemErro           bool   `json:"temErro"`
		Exercicio         string `json:"exercicio"`
		SituacaoDocumento string `json:"situacaoDocumento"`
		DescDocumento     string `json:"descDocumento"`
	} `json:"licenciamento"`

	Imposto struct {
		TemErro   bool `json:"temErro"`
		Historico []struct {
			Exercicio      string  `json:"exercicio"`
			Situacao       string  `json:"situacao"`
			DataVencimento *string `json:"dataVencimento"`
			ValorOriginal  string  `json:"valorOriginal"`
			DividaAtiva    bool    `json:"dividaAtiva"`
		} `json:"historico"`
	} `json:"imposto"`

	Restricao struct {
		TemErro    bool `json:"temErro"`
		Restricoes []struct {
			Tipo      string `json:"tipo"`
			Descricao string `json:"descricao"`
		} `json:"restricoes"`
	} `json:"restricao"`

	Seguro struct {
		TemErro            bool   `json:"temErro"`
		ExercicioAtual     string `json:"exercicioAtual"`
		ValorExercAtual    string `json:"valorExercAtual"`
		SituacaoExercAtual string `json:"situacaoExercAtual"`
	} `json:"seguro"`

	Infracao struct {
		TemErro       bool   `json:"temErro"`
		QtAgPrazoDef  int    `json:"qtAgPrazoDef"`
		VlAgPrazoDef  string `json:"vlAgPrazoDef"`
		QtAgPrazoJulg int    `json:"qtAgPrazoJulg"`
		VlAgPrazoJulg string `json:"vlAgPrazoJulg"`
		QtAVencer     int    `json:"qtAVencer"`
		VlAVencer     string `json:"vlAVencer"`
		QtSuspensas   int    `json:"qtSuspensas"`
		VlSuspensas   string `json:"vlSuspensas"`
		QtVencidas    int    `json:"qtVencidas"`
		VlVencidas    string `json:"vlVencidas"`
	} `json:"infracao"`
}

func ParseVehicle(raw []byte) (*model.QueryResponse, error) {
	var r apiResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("detranrs: parse response: %w", err)
	}

	id := r.Identificacao
	resp := &model.QueryResponse{
		Plate:  id.Placa,
		Source: "DETRAN-RS",
		Vehicle: &model.Vehicle{
			Plate:           id.Placa,
			Renavam:         renavamString(id.Renavam),
			Chassis:         id.Chassi,
			MakeModel:       id.MarcaModelo,
			ManufactureYear: id.AnoFabricacao,
			ModelYear:       id.AnoModelo,
			Color:           id.Cor,
			Type:            id.Tipo,
			Species:         id.Especie,
			Category:        id.Categoria,
			City:            id.MunicipioRegistro,
			PlateState:      id.UfPlaca,
			Fuel:            id.Combustivel,
			RenavamStatus:   id.Situacao,
			OwnerCPF:        id.CpfProprietario,
		},
	}

	if r.Licenciamento.Exercicio != "" || r.Licenciamento.SituacaoDocumento != "" {
		resp.Licensing = &model.Licensing{
			Year:           r.Licenciamento.Exercicio,
			DocumentStatus: r.Licenciamento.SituacaoDocumento,
			Document:       r.Licenciamento.DescDocumento,
			DueDate:        id.DtVencLicenciamento,
		}
	}

	for _, h := range r.Imposto.Historico {
		amount := normalizeBRLAmount(h.ValorOriginal)
		if isExempt(h.Situacao) || amount == "0.00" {
			continue
		}
		resp.Debts = append(resp.Debts, model.Debt{
			Type:    "IPVA",
			Year:    safeAtoi(h.Exercicio),
			Amount:  amount,
			DueDate: deref(h.DataVencimento),
		})
	}

	for _, rr := range r.Restricao.Restricoes {
		resp.Restrictions = append(resp.Restrictions, model.Restriction{
			Type:        rr.Tipo,
			Description: rr.Descricao,
		})
	}

	if id.Furtado {
		resp.Restrictions = append(resp.Restrictions, model.Restriction{
			Type:        "THEFT_ROBBERY",
			Description: "Vehicle reported stolen/robbed",
		})
	}
	if id.EmDeposito {
		resp.Restrictions = append(resp.Restrictions, model.Restriction{
			Type:        "IMPOUNDED",
			Description: "Vehicle impounded",
		})
	}

	if s := r.Seguro; !isExempt(s.SituacaoExercAtual) {
		if amount := normalizeBRLAmount(s.ValorExercAtual); amount != "0.00" {
			resp.Debts = append(resp.Debts, model.Debt{
				Type:   "DPVAT",
				Year:   safeAtoi(s.ExercicioAtual),
				Amount: amount,
			})
		}
	}

	inf := r.Infracao
	total := inf.QtAVencer + inf.QtVencidas + inf.QtSuspensas + inf.QtAgPrazoDef + inf.QtAgPrazoJulg
	if total > 0 {
		resp.Violations = &model.Violations{
			Upcoming:         model.ViolationSummary{Count: inf.QtAVencer, Amount: normalizeBRLAmount(inf.VlAVencer)},
			Overdue:          model.ViolationSummary{Count: inf.QtVencidas, Amount: normalizeBRLAmount(inf.VlVencidas)},
			Suspended:        model.ViolationSummary{Count: inf.QtSuspensas, Amount: normalizeBRLAmount(inf.VlSuspensas)},
			AwaitingDefense:  model.ViolationSummary{Count: inf.QtAgPrazoDef, Amount: normalizeBRLAmount(inf.VlAgPrazoDef)},
			AwaitingJudgment: model.ViolationSummary{Count: inf.QtAgPrazoJulg, Amount: normalizeBRLAmount(inf.VlAgPrazoJulg)},
		}
	}

	return resp, nil
}

func normalizeBRLAmount(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "0.00"
	}
	s = strings.ReplaceAll(s, "R$", "")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return "0.00"
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return s
	}
	return s
}

func isExempt(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "isento" || s == "não devido" || s == "nao devido"
}

func renavamString(r int64) string {
	if r == 0 {
		return ""
	}
	return strconv.FormatInt(r, 10)
}

func safeAtoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
