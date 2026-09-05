package detranrs

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

// TestParseVeiculo valida o parser contra a fixture JSON (anonimizada) do
// DETRAN-RS, cobrindo dados cadastrais, licenciamento e extração de IPVA.
func TestParseVeiculo(t *testing.T) {
	path := filepath.Join("..", "..", "..", "fixtures", "detranrs_veiculo.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lendo fixture %s: %v", path, err)
	}

	resp, err := ParseVeiculo(raw)
	if err != nil {
		t.Fatalf("ParseVeiculo falhou: %v", err)
	}

	// --- Dados cadastrais ---
	v := resp.Veiculo
	if v == nil {
		t.Fatal("veiculo nil")
	}
	casos := []struct {
		campo, got, want string
	}{
		{"placa", v.Placa, "ABC1D23"},
		{"renavam", v.Renavam, "123456789"},
		{"chassi", v.Chassi, "9BWZZZ00ZZZ000000"},
		{"marcaModelo", v.MarcaModelo, "VW/FUSCA"},
		{"cor", v.Cor, "Bege"},
		{"tipo", v.Tipo, "Automóvel"},
		{"especie", v.Especie, "Passageiro"},
		{"municipio", v.Municipio, "PORTO ALEGRE"},
		{"ufPlaca", v.UfPlaca, "RS"},
		{"situacaoRenavam", v.SituacaoRenavam, "Em circulação"},
	}
	for _, c := range casos {
		if c.got != c.want {
			t.Errorf("%s = %q; quero %q", c.campo, c.got, c.want)
		}
	}
	if v.AnoFabricacao != 1986 || v.AnoModelo != 1986 {
		t.Errorf("ano fab/modelo = %d/%d; quero 1986/1986", v.AnoFabricacao, v.AnoModelo)
	}

	// --- Licenciamento ---
	if resp.Licenciamento == nil {
		t.Fatal("licenciamento nil")
	}
	if resp.Licenciamento.Documento != "CRLV" {
		t.Errorf("documento = %q; quero CRLV", resp.Licenciamento.Documento)
	}
	if resp.Licenciamento.DataVencimento != "31/07/2027" {
		t.Errorf("dtVencLicenciamento = %q; quero 31/07/2027", resp.Licenciamento.DataVencimento)
	}

	// --- Débitos: só o IPVA devido de 2026 entra (isentos são ignorados) ---
	if len(resp.Debitos) != 1 {
		t.Fatalf("esperava 1 débito (IPVA devido), veio %d: %+v", len(resp.Debitos), resp.Debitos)
	}
	d := resp.Debitos[0]
	if d.Tipo != "IPVA" || d.Exercicio != 2026 {
		t.Errorf("débito = %s/%d; quero IPVA/2026", d.Tipo, d.Exercicio)
	}
	if d.Valor != "1234.56" {
		t.Errorf("valor normalizado = %q; quero 1234.56", d.Valor)
	}
	if d.Vencimento != "31/03/2026" {
		t.Errorf("vencimento = %q; quero 31/03/2026", d.Vencimento)
	}

	// --- Restrições: fixture tem restricoes=null ---
	if len(resp.Restricoes) != 0 {
		t.Errorf("esperava 0 restrições, veio %d", len(resp.Restricoes))
	}
}

// TestParseVeiculoComPendencias exercita as seções de débitos (IPVA + DPVAT),
// infrações e restrições (incluindo roubo/furto) num veículo forjado.
// NOTA: o formato do array restricao.restricoes é uma HIPÓTESE (veio null no
// fixture real) — confirmar contra um veículo realmente restrito.
func TestParseVeiculoComPendencias(t *testing.T) {
	path := filepath.Join("..", "..", "..", "fixtures", "detranrs_veiculo_pendencias.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lendo fixture %s: %v", path, err)
	}
	resp, err := ParseVeiculo(raw)
	if err != nil {
		t.Fatalf("ParseVeiculo falhou: %v", err)
	}

	// --- Débitos: IPVA 2026, IPVA 2025 e DPVAT ---
	debitosPorTipo := map[string]model.Debito{}
	var qtdIPVA int
	for _, d := range resp.Debitos {
		if d.Tipo == "IPVA" {
			qtdIPVA++
		}
		debitosPorTipo[d.Tipo+"-"+itoa(d.Exercicio)] = d
	}
	if qtdIPVA != 2 {
		t.Errorf("esperava 2 débitos de IPVA (2026 e 2025), veio %d", qtdIPVA)
	}
	if dpvat, ok := debitosPorTipo["DPVAT-2026"]; !ok {
		t.Error("esperava débito DPVAT 2026")
	} else if dpvat.Valor != "105.65" {
		t.Errorf("DPVAT valor = %q; quero 105.65", dpvat.Valor)
	}

	// --- Infrações (agregados) ---
	if resp.Infracoes == nil {
		t.Fatal("infracoes nil (esperava agregados)")
	}
	if resp.Infracoes.Vencidas.Quantidade != 2 || resp.Infracoes.Vencidas.Valor != "293.47" {
		t.Errorf("vencidas = %+v; quero {2, 293.47}", resp.Infracoes.Vencidas)
	}
	if resp.Infracoes.AVencer.Quantidade != 1 || resp.Infracoes.AVencer.Valor != "130.16" {
		t.Errorf("aVencer = %+v; quero {1, 130.16}", resp.Infracoes.AVencer)
	}

	// --- Restrições: a administrativa + a de roubo/furto (furtado=true) ---
	tipos := map[string]bool{}
	for _, r := range resp.Restricoes {
		tipos[r.Tipo] = true
	}
	if !tipos["ADMINISTRATIVA"] {
		t.Error("esperava restrição ADMINISTRATIVA")
	}
	if !tipos["ROUBO_FURTO"] {
		t.Error("esperava restrição ROUBO_FURTO (furtado=true)")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestNormalizeValorBRL(t *testing.T) {
	casos := map[string]string{
		"R$ 0,00":      "0.00",
		"R$ 1.234,56":  "1234.56",
		"R$ 10,50":     "10.50",
		"R$ 1.000.000,00": "1000000.00",
		"":             "0.00",
		"  R$  99,90 ": "99.90",
	}
	for in, want := range casos {
		if got := normalizeValorBRL(in); got != want {
			t.Errorf("normalizeValorBRL(%q) = %q; quero %q", in, got, want)
		}
	}
}
