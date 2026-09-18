package detranrs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

// apiResponse mirrors the JSON of
// GET /pcsdetran/rest/veiculos/{plate}/?renavam=...&contabiliza=false
//
// A single call returns registration, licensing, IPVA, insurance, violations
// and restrictions. The JSON tags stay in Portuguese because they are the
// external DETRAN API format; only the Go/output names are in English.
type apiResponse struct {
	Registration struct {
		HasError            bool   `json:"temErro"`
		MakeModel           string `json:"marcaModelo"`
		Renavam             int64  `json:"renavam"`
		PlateState          string `json:"ufPlaca"`
		RegistrationCity    string `json:"municipioRegistro"`
		Type                string `json:"tipo"`
		Color               string `json:"cor"`
		Species             string `json:"especie"`
		Plate               string `json:"placa"`
		Chassis             string `json:"chassi"`
		Situation           string `json:"situacao"`
		OwnerCPF            string `json:"cpfProprietario"`
		Fuel                string `json:"combustivel"`
		Category            string `json:"categoria"`
		ManufactureYear     int    `json:"anoFabricacao"`
		ModelYear           int    `json:"anoModelo"`
		LicensingDueDate    string `json:"dtVencLicenciamento"`
		Stolen              bool   `json:"furtado"`
		Impounded           bool   `json:"emDeposito"`
	} `json:"identificacao"`

	Licensing struct {
		HasError       bool   `json:"temErro"`
		Year           string `json:"exercicio"`
		DocumentStatus string `json:"situacaoDocumento"`
		DocumentDesc   string `json:"descDocumento"`
	} `json:"licenciamento"`

	Tax struct {
		HasError bool `json:"temErro"`
		History  []struct {
			Year        string  `json:"exercicio"`
			Situation   string  `json:"situacao"`
			DueDate     *string `json:"dataVencimento"`
			OrigAmount  string  `json:"valorOriginal"`
			ActiveDebt  bool    `json:"dividaAtiva"`
		} `json:"historico"`
	} `json:"imposto"`

	Restriction struct {
		HasError     bool `json:"temErro"`
		Restrictions []struct {
			// TODO: confirm the real fields once a vehicle with a restriction
			// appears (came null in the real fixture).
			Type        string `json:"tipo"`
			Description string `json:"descricao"`
		} `json:"restricoes"`
	} `json:"restricao"`

	// Insurance = DPVAT.
	Insurance struct {
		HasError        bool   `json:"temErro"`
		CurrentYear     string `json:"exercicioAtual"`
		CurrentAmount   string `json:"valorExercAtual"`
		CurrentStatus   string `json:"situacaoExercAtual"`
	} `json:"seguro"`

	// Violations carries fine aggregates by status (not individual items).
	Violations struct {
		HasError          bool   `json:"temErro"`
		QtyAwaitDefense   int    `json:"qtAgPrazoDef"`
		AmtAwaitDefense   string `json:"vlAgPrazoDef"`
		QtyAwaitJudgment  int    `json:"qtAgPrazoJulg"`
		AmtAwaitJudgment  string `json:"vlAgPrazoJulg"`
		QtyUpcoming       int    `json:"qtAVencer"`
		AmtUpcoming       string `json:"vlAVencer"`
		QtySuspended      int    `json:"qtSuspensas"`
		AmtSuspended      string `json:"vlSuspensas"`
		QtyOverdue        int    `json:"qtVencidas"`
		AmtOverdue        string `json:"vlVencidas"`
	} `json:"infracao"`
}

// ParseVehicle converts the raw API JSON into a normalized QueryResponse.
// It does NOT touch the network — takes bytes, returns a struct. Testable with fixtures.
func ParseVehicle(raw []byte) (*model.QueryResponse, error) {
	var r apiResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("detranrs: parse response: %w", err)
	}

	reg := r.Registration
	resp := &model.QueryResponse{
		Plate:  reg.Plate,
		Source: "DETRAN-RS",
		Vehicle: &model.Vehicle{
			Plate:           reg.Plate,
			Renavam:         renavamString(reg.Renavam),
			Chassis:         reg.Chassis,
			MakeModel:       reg.MakeModel,
			ManufactureYear: reg.ManufactureYear,
			ModelYear:       reg.ModelYear,
			Color:           reg.Color,
			Type:            reg.Type,
			Species:         reg.Species,
			Category:        reg.Category,
			City:            reg.RegistrationCity,
			PlateState:      reg.PlateState,
			Fuel:            reg.Fuel,
			RenavamStatus:   reg.Situation,
			OwnerCPF:        reg.OwnerCPF,
		},
	}

	// Licensing (CRLV).
	if r.Licensing.Year != "" || r.Licensing.DocumentStatus != "" {
		resp.Licensing = &model.Licensing{
			Year:           r.Licensing.Year,
			DocumentStatus: r.Licensing.DocumentStatus,
			Document:       r.Licensing.DocumentDesc,
			DueDate:        reg.LicensingDueDate,
		}
	}

	// IPVA history: expose EVERY year with its status ("Isento", "Liquidado",
	// "Devido", ...). Owed years also go into debts (actionable).
	for _, h := range r.Tax.History {
		amount := normalizeBRLAmount(h.OrigAmount)
		dueDate := deref(h.DueDate)
		resp.Taxes = append(resp.Taxes, model.TaxEntry{
			Year:       h.Year,
			Status:     h.Situation,
			Amount:     amount,
			DueDate:    dueDate,
			ActiveDebt: h.ActiveDebt,
		})
		if isOwedTax(h.Situation, h.ActiveDebt) && amount != "0.00" {
			resp.Debts = append(resp.Debts, model.Debt{
				Type:    "IPVA",
				Year:    safeAtoi(h.Year),
				Amount:  amount,
				DueDate: dueDate,
			})
		}
	}

	// Restrictions.
	for _, rr := range r.Restriction.Restrictions {
		resp.Restrictions = append(resp.Restrictions, model.Restriction{
			Type:        rr.Type,
			Description: rr.Description,
		})
	}

	// Theft/robbery and impound become restrictions (flags in registration).
	if reg.Stolen {
		resp.Restrictions = append(resp.Restrictions, model.Restriction{
			Type:        "THEFT_ROBBERY",
			Description: "Vehicle reported stolen/robbed",
		})
	}
	if reg.Impounded {
		resp.Restrictions = append(resp.Restrictions, model.Restriction{
			Type:        "IMPOUNDED",
			Description: "Vehicle impounded",
		})
	}

	// DPVAT (insurance): a debt when owed.
	if ins := r.Insurance; !isExempt(ins.CurrentStatus) {
		if amount := normalizeBRLAmount(ins.CurrentAmount); amount != "0.00" {
			resp.Debts = append(resp.Debts, model.Debt{
				Type:   "DPVAT",
				Year:   safeAtoi(ins.CurrentYear),
				Amount: amount,
			})
		}
	}

	// Violations (aggregates by status). Included only if there are any.
	v := r.Violations
	total := v.QtyUpcoming + v.QtyOverdue + v.QtySuspended + v.QtyAwaitDefense + v.QtyAwaitJudgment
	if total > 0 {
		resp.Violations = &model.Violations{
			Upcoming:         model.ViolationSummary{Count: v.QtyUpcoming, Amount: normalizeBRLAmount(v.AmtUpcoming)},
			Overdue:          model.ViolationSummary{Count: v.QtyOverdue, Amount: normalizeBRLAmount(v.AmtOverdue)},
			Suspended:        model.ViolationSummary{Count: v.QtySuspended, Amount: normalizeBRLAmount(v.AmtSuspended)},
			AwaitingDefense:  model.ViolationSummary{Count: v.QtyAwaitDefense, Amount: normalizeBRLAmount(v.AmtAwaitDefense)},
			AwaitingJudgment: model.ViolationSummary{Count: v.QtyAwaitJudgment, Amount: normalizeBRLAmount(v.AmtAwaitJudgment)},
		}
	}

	return resp, nil
}

// normalizeBRLAmount converts "R$ 1.234,56" -> "1234.56" (decimal for BigDecimal).
// Empty/invalid input becomes "0.00".
func normalizeBRLAmount(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "0.00"
	}
	s = strings.ReplaceAll(s, "R$", "")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", "")  // thousands separator
	s = strings.ReplaceAll(s, ",", ".") // decimal separator
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

// isOwedTax reports whether an IPVA history entry is actually owed. Statuses
// like "Isento", "Liquidado" (settled) and "Pago" are NOT owed. An entry flagged
// as dívida ativa (activeDebt) is always owed.
func isOwedTax(status string, activeDebt bool) bool {
	if activeDebt {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "devido", "em aberto", "vencido", "em atraso", "pendente", "a pagar":
		return true
	}
	return false
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
