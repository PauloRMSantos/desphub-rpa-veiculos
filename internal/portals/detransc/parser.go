package detransc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

type dossieResponse struct {
	Plate           string   `json:"placa"`
	Renavam         int64    `json:"renavam"`
	PreviousPlate   string   `json:"placaAnterior"`
	OwnerCurrent    string   `json:"proprietarioAtual"`
	Color           string   `json:"cor"`
	MakeModel       string   `json:"marcaModelo"`
	ManufactureYear int      `json:"anoFabricacao"`
	ModelYear       int      `json:"anoModelo"`
	Status          string   `json:"motivoBaixa"` 
	LicensingYear   int      `json:"exercicioLicenciamento"`
	Licensed        string   `json:"licenciado"` 
	City            string   `json:"municipio"`
	UF              string   `json:"uf"`
	Type            string   `json:"tipo"`
	Category        string   `json:"categoria"`
	Species         string   `json:"especie"`
	Fuel            string   `json:"combustivel"`
	SaleRestriction string   `json:"restricaoVenda"` 
	SngPending      []string `json:"informacoesPendentesSng"`

	Violations       []scViolation `json:"infracoes"`
	ViolationHistory []scViolation `json:"historicoInfracoes"`

	Debts      []scDebt `json:"debitos"`
	ActiveDebt []scDebt `json:"dividaAtiva"`
}

type scViolation struct {
	Notice      string  `json:"numeroAuto"`
	Description string  `json:"descricao"`
	DateTime    string  `json:"dataHoraAutuacao"`
	Location    string  `json:"localInfracao"`
	Amount      float64 `json:"valor"`
	Situation   string  `json:"situacao"`
	Status      string  `json:"status"`
}

type scDebt struct {
	Description string  `json:"descricao"`
	Amount      float64 `json:"valor"`
	Year        int     `json:"exercicio"`
	DueDate     string  `json:"dataVencimento"`
}

func ParseVehicle(raw []byte) (*model.QueryResponse, error) {
	var r dossieResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("detransc: parse response: %w", err)
	}

	resp := &model.QueryResponse{
		Plate:  r.Plate,
		Source: "DETRAN-SC",
		Vehicle: &model.Vehicle{
			Plate:           r.Plate,
			PreviousPlate:   strings.TrimSpace(r.PreviousPlate),
			Renavam:         renavamString(r.Renavam),
			MakeModel:       r.MakeModel,
			ManufactureYear: r.ManufactureYear,
			ModelYear:       r.ModelYear,
			Color:           r.Color,
			Type:            r.Type,
			Species:         r.Species,
			Category:        r.Category,
			Fuel:            r.Fuel,
			City:            r.City,
			PlateState:      r.UF,
			RenavamStatus:   r.Status,
			OwnerName:       strings.TrimSpace(r.OwnerCurrent),
		},
	}

	if r.LicensingYear != 0 || r.Licensed != "" {
		resp.Licensing = &model.Licensing{
			Year:           strconv.Itoa(r.LicensingYear),
			DocumentStatus: strings.TrimSpace(r.Licensed),
			Document:       "CRLV",
		}
	}

	for _, v := range r.Violations {
		resp.Fines = append(resp.Fines, toFine(v))
	}
	for _, v := range r.ViolationHistory {
		resp.Fines = append(resp.Fines, toFine(v))
	}

	for _, d := range r.Debts {
		resp.Debts = append(resp.Debts, model.Debt{
			Type:    strings.TrimSpace(d.Description),
			Year:    d.Year,
			Amount:  decimalString(d.Amount),
			DueDate: d.DueDate,
		})
	}
	for _, d := range r.ActiveDebt {
		resp.Debts = append(resp.Debts, model.Debt{
			Type:   "DIVIDA_ATIVA " + strings.TrimSpace(d.Description),
			Year:   d.Year,
			Amount: decimalString(d.Amount),
		})
	}

	if s := strings.TrimSpace(r.SaleRestriction); s != "" {
		resp.Restrictions = append(resp.Restrictions, model.Restriction{
			Type:        "SALE_RESTRICTION",
			Description: s,
		})
	}
	for _, s := range r.SngPending {
		if s = strings.TrimSpace(s); s != "" {
			resp.Restrictions = append(resp.Restrictions, model.Restriction{
				Type:        "SNG_PENDING",
				Description: s,
			})
		}
	}

	return resp, nil
}

func toFine(v scViolation) model.Fine {
	return model.Fine{
		Notice:      strings.TrimSpace(v.Notice),
		Description: strings.TrimSpace(v.Description),
		Date:        v.DateTime,
		Location:    strings.TrimSpace(v.Location),
		Amount:      decimalString(v.Amount),
		Situation:   strings.TrimSpace(v.Situation),
		Status:      strings.TrimSpace(v.Status),
	}
}

func decimalString(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func renavamString(r int64) string {
	if r == 0 {
		return ""
	}
	return strconv.FormatInt(r, 10)
}
