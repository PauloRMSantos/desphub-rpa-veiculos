package detranrs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

type apiResposta struct {
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
		TemErro          bool   `json:"temErro"`
		Exercicio        string `json:"exercicio"`
		SituacaoDocumento string `json:"situacaoDocumento"`
		DescDocumento    string `json:"descDocumento"`
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
			// TODO: confirmar campos reais quando houver um veículo com
			// restrição (veio null no fixture real)
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
		TemErro      bool   `json:"temErro"`
		QtAgPrazoDef int    `json:"qtAgPrazoDef"`
		VlAgPrazoDef string `json:"vlAgPrazoDef"`
		QtAgPrazoJulg int   `json:"qtAgPrazoJulg"`
		VlAgPrazoJulg string `json:"vlAgPrazoJulg"`
		QtAVencer    int    `json:"qtAVencer"`
		VlAVencer    string `json:"vlAVencer"`
		QtSuspensas  int    `json:"qtSuspensas"`
		VlSuspensas  string `json:"vlSuspensas"`
		QtVencidas   int    `json:"qtVencidas"`
		VlVencidas   string `json:"vlVencidas"`
	} `json:"infracao"`
}

func ParseVeiculo(raw []byte) (*model.ConsultaResponse, error) {
	var r apiResposta
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("detranrs: parse resposta: %w", err)
	}

	id := r.Identificacao
	resp := &model.ConsultaResponse{
		Placa: id.Placa,
		Fonte: "DETRAN-RS",
		Veiculo: &model.Veiculo{
			Placa:           id.Placa,
			Renavam:         renavamStr(id.Renavam),
			Chassi:          id.Chassi,
			MarcaModelo:     id.MarcaModelo,
			AnoFabricacao:   id.AnoFabricacao,
			AnoModelo:       id.AnoModelo,
			Cor:             id.Cor,
			Tipo:            id.Tipo,
			Especie:         id.Especie,
			Categoria:       id.Categoria,
			Municipio:       id.MunicipioRegistro,
			UfPlaca:         id.UfPlaca,
			Combustivel:     id.Combustivel,
			SituacaoRenavam: id.Situacao,
			CpfProprietario: id.CpfProprietario,
		},
	}

	if r.Licenciamento.Exercicio != "" || r.Licenciamento.SituacaoDocumento != "" {
		resp.Licenciamento = &model.Licenciamento{
			Exercicio:         r.Licenciamento.Exercicio,
			SituacaoDocumento: r.Licenciamento.SituacaoDocumento,
			Documento:         r.Licenciamento.DescDocumento,
			DataVencimento:    id.DtVencLicenciamento,
		}
	}

	for _, h := range r.Imposto.Historico {
		valor := normalizeValorBRL(h.ValorOriginal)
		if ehIsento(h.Situacao) || valor == "0.00" {
			continue
		}
		resp.Debitos = append(resp.Debitos, model.Debito{
			Tipo:       "IPVA",
			Exercicio:  atoiSafe(h.Exercicio),
			Valor:      valor,
			Vencimento: deref(h.DataVencimento),
		})
	}

	for _, rr := range r.Restricao.Restricoes {
		resp.Restricoes = append(resp.Restricoes, model.Restricao{
			Tipo:      rr.Tipo,
			Descricao: rr.Descricao,
		})
	}

	if id.Furtado {
		resp.Restricoes = append(resp.Restricoes, model.Restricao{
			Tipo:      "ROUBO_FURTO",
			Descricao: "Veículo com registro de roubo/furto",
		})
	}
	if id.EmDeposito {
		resp.Restricoes = append(resp.Restricoes, model.Restricao{
			Tipo:      "EM_DEPOSITO",
			Descricao: "Veículo em depósito",
		})
	}

	if s := r.Seguro; !ehIsento(s.SituacaoExercAtual) {
		if valor := normalizeValorBRL(s.ValorExercAtual); valor != "0.00" {
			resp.Debitos = append(resp.Debitos, model.Debito{
				Tipo:      "DPVAT",
				Exercicio: atoiSafe(s.ExercicioAtual),
				Valor:     valor,
			})
		}
	}

	inf := r.Infracao
	total := inf.QtAVencer + inf.QtVencidas + inf.QtSuspensas + inf.QtAgPrazoDef + inf.QtAgPrazoJulg
	if total > 0 {
		resp.Infracoes = &model.Infracoes{
			AVencer:               model.ResumoInfracao{Quantidade: inf.QtAVencer, Valor: normalizeValorBRL(inf.VlAVencer)},
			Vencidas:              model.ResumoInfracao{Quantidade: inf.QtVencidas, Valor: normalizeValorBRL(inf.VlVencidas)},
			Suspensas:             model.ResumoInfracao{Quantidade: inf.QtSuspensas, Valor: normalizeValorBRL(inf.VlSuspensas)},
			AguardandoPrazoDefesa: model.ResumoInfracao{Quantidade: inf.QtAgPrazoDef, Valor: normalizeValorBRL(inf.VlAgPrazoDef)},
			AguardandoJulgamento:  model.ResumoInfracao{Quantidade: inf.QtAgPrazoJulg, Valor: normalizeValorBRL(inf.VlAgPrazoJulg)},
		}
	}

	return resp, nil
}


func normalizeValorBRL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "0.00"
	}
	s = strings.ReplaceAll(s, "R$", "")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", "") // separador de milhar
	s = strings.ReplaceAll(s, ",", ".") // separador decimal
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return "0.00"
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return s
	}
	return s
}

func ehIsento(situacao string) bool {
	s := strings.ToLower(strings.TrimSpace(situacao))
	return s == "isento" || s == "não devido" || s == "nao devido"
}

func renavamStr(r int64) string {
	if r == 0 {
		return ""
	}
	return strconv.FormatInt(r, 10)
}

func atoiSafe(s string) int {
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
