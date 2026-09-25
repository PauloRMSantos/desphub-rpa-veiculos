package detranrs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

// TestParseVehicle validates the parser against the (anonymized) DETRAN-RS JSON
// fixture, covering registration data, licensing and IPVA extraction.
func TestParseVehicle(t *testing.T) {
	path := filepath.Join("..", "..", "..", "fixtures", "detranrs_vehicle_clean.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}

	resp, err := ParseVehicle(raw)
	if err != nil {
		t.Fatalf("ParseVehicle failed: %v", err)
	}

	// --- Registration data ---
	v := resp.Vehicle
	if v == nil {
		t.Fatal("vehicle nil")
	}
	cases := []struct {
		field, got, want string
	}{
		{"plate", v.Plate, "ABC1D23"},
		{"renavam", v.Renavam, "123456789"},
		{"chassis", v.Chassis, "9BWZZZ00ZZZ000000"},
		{"makeModel", v.MakeModel, "VW/FUSCA"},
		{"color", v.Color, "Bege"},
		{"type", v.Type, "Automóvel"},
		{"species", v.Species, "Passageiro"},
		{"city", v.City, "PORTO ALEGRE"},
		{"plateState", v.PlateState, "RS"},
		{"renavamStatus", v.RenavamStatus, "Em circulação"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q; want %q", c.field, c.got, c.want)
		}
	}
	if v.ManufactureYear != 1986 || v.ModelYear != 1986 {
		t.Errorf("manufacture/model year = %d/%d; want 1986/1986", v.ManufactureYear, v.ModelYear)
	}

	// --- Licensing ---
	if resp.Licensing == nil {
		t.Fatal("licensing nil")
	}
	if resp.Licensing.Document != "CRLV" {
		t.Errorf("document = %q; want CRLV", resp.Licensing.Document)
	}
	if resp.Licensing.DueDate != "31/07/2027" {
		t.Errorf("licensing dueDate = %q; want 31/07/2027", resp.Licensing.DueDate)
	}

	// --- Debts: only the owed 2026 IPVA (exempt ones skipped) ---
	if len(resp.Debts) != 1 {
		t.Fatalf("expected 1 debt (owed IPVA), got %d: %+v", len(resp.Debts), resp.Debts)
	}
	d := resp.Debts[0]
	if d.Type != "IPVA" || d.Year != 2026 {
		t.Errorf("debt = %s/%d; want IPVA/2026", d.Type, d.Year)
	}
	if d.Amount != "1234.56" {
		t.Errorf("normalized amount = %q; want 1234.56", d.Amount)
	}
	if d.DueDate != "31/03/2026" {
		t.Errorf("dueDate = %q; want 31/03/2026", d.DueDate)
	}

	// --- Taxes: FULL IPVA history with status (all years, not only owed) ---
	if len(resp.Taxes) != 4 {
		t.Fatalf("expected 4 tax history entries, got %d: %+v", len(resp.Taxes), resp.Taxes)
	}
	taxByYear := map[string]model.TaxEntry{}
	for _, tx := range resp.Taxes {
		taxByYear[tx.Year] = tx
	}
	if tx := taxByYear["2026"]; tx.Status != "Devido" || tx.Amount != "1234.56" {
		t.Errorf("tax 2026 = %+v; want status Devido / 1234.56", tx)
	}
	if tx := taxByYear["2025"]; tx.Status != "Isento" || tx.Amount != "0.00" {
		t.Errorf("tax 2025 = %+v; want status Isento / 0.00", tx)
	}
	// "Liquidado" (settled) shows in taxes but must NOT be counted as an owed debt.
	if tx := taxByYear["2023"]; tx.Status != "Liquidado" || tx.Amount != "798.05" {
		t.Errorf("tax 2023 = %+v; want status Liquidado / 798.05", tx)
	}

	// --- Restrictions: fixture has restricoes=null ---
	if len(resp.Restrictions) != 0 {
		t.Errorf("expected 0 restrictions, got %d", len(resp.Restrictions))
	}
}

// TestParseVehicleWithPending exercises the debts (IPVA + DPVAT), violations and
// restrictions (including theft/robbery) sections on a crafted vehicle.
// NOTE: the restricao.restricoes[] shape is a HYPOTHESIS (came null in the real
// fixture) — confirm against a truly restricted vehicle.
func TestParseVehicleWithPending(t *testing.T) {
	path := filepath.Join("..", "..", "..", "fixtures", "detranrs_vehicle_with_pending.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	resp, err := ParseVehicle(raw)
	if err != nil {
		t.Fatalf("ParseVehicle failed: %v", err)
	}

	// --- Debts: IPVA 2026, IPVA 2025 and DPVAT ---
	debtsByKey := map[string]model.Debt{}
	var ipvaCount int
	for _, d := range resp.Debts {
		if d.Type == "IPVA" {
			ipvaCount++
		}
		debtsByKey[d.Type+"-"+strconv.Itoa(d.Year)] = d
	}
	if ipvaCount != 2 {
		t.Errorf("expected 2 IPVA debts (2026 and 2025), got %d", ipvaCount)
	}
	if dpvat, ok := debtsByKey["DPVAT-2026"]; !ok {
		t.Error("expected DPVAT 2026 debt")
	} else if dpvat.Amount != "105.65" {
		t.Errorf("DPVAT amount = %q; want 105.65", dpvat.Amount)
	}

	// --- Violations (aggregates) ---
	if resp.Violations == nil {
		t.Fatal("violations nil (expected aggregates)")
	}
	if resp.Violations.Overdue.Count != 2 || resp.Violations.Overdue.Amount != "293.47" {
		t.Errorf("overdue = %+v; want {2, 293.47}", resp.Violations.Overdue)
	}
	if resp.Violations.Upcoming.Count != 1 || resp.Violations.Upcoming.Amount != "130.16" {
		t.Errorf("upcoming = %+v; want {1, 130.16}", resp.Violations.Upcoming)
	}

	// --- Restrictions: the administrative one + the theft/robbery (furtado=true) ---
	types := map[string]bool{}
	for _, r := range resp.Restrictions {
		types[r.Type] = true
	}
	if !types["ADMINISTRATIVA"] {
		t.Error("expected ADMINISTRATIVA restriction")
	}
	if !types["THEFT_ROBBERY"] {
		t.Error("expected THEFT_ROBBERY restriction (furtado=true)")
	}

	// --- Special characteristics: "Recuperado de sinistro" (salvage) ---
	if len(resp.SpecialCharacteristics) != 1 {
		t.Fatalf("expected 1 special characteristic, got %d: %+v", len(resp.SpecialCharacteristics), resp.SpecialCharacteristics)
	}
	sc := resp.SpecialCharacteristics[0]
	if !strings.Contains(sc.Description, "Recuperado de sinistro") {
		t.Errorf("special characteristic description = %q; want it to mention 'Recuperado de sinistro'", sc.Description)
	}
	if sc.Code != "15360601302025" {
		t.Errorf("special characteristic code = %q; want 15360601302025", sc.Code)
	}

	// --- "Em Divida Ativa Liquidada/Concluida" (settled) must NOT be a debt ---
	// ipvaCount stayed 2 (2026 Devido + 2025 dividaAtiva), so the settled active
	// debt from 2023 was correctly excluded from debts.
	if ipvaCount != 2 {
		t.Errorf("settled 'Em Divida Ativa Liquidada/Concluida' wrongly counted as debt (ipvaCount=%d)", ipvaCount)
	}
}

func TestNormalizeBRLAmount(t *testing.T) {
	cases := map[string]string{
		"R$ 0,00":         "0.00",
		"R$ 1.234,56":     "1234.56",
		"R$ 10,50":        "10.50",
		"R$ 1.000.000,00": "1000000.00",
		"":                "0.00",
		"  R$  99,90 ":    "99.90",
	}
	for in, want := range cases {
		if got := normalizeBRLAmount(in); got != want {
			t.Errorf("normalizeBRLAmount(%q) = %q; want %q", in, got, want)
		}
	}
}
