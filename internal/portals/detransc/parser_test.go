package detransc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

// TestParseVehicle validates the DETRAN-SC dossiê parser against the (masked)
// reference payload.
func TestParseVehicle(t *testing.T) {
	path := filepath.Join("..", "..", "..", "fixtures", "detransc_dossie.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}

	resp, err := ParseVehicle(raw)
	if err != nil {
		t.Fatalf("ParseVehicle failed: %v", err)
	}
	if resp.Source != "DETRAN-SC" {
		t.Errorf("source = %q; want DETRAN-SC", resp.Source)
	}

	// --- Registration ---
	v := resp.Vehicle
	if v == nil {
		t.Fatal("vehicle nil")
	}
	cases := []struct{ field, got, want string }{
		{"plate", v.Plate, "RAC9J36"},
		{"previousPlate", v.PreviousPlate, "RAC9936"},
		{"renavam", v.Renavam, "1204058129"},
		{"makeModel", v.MakeModel, "HONDA/CB 500X"},
		{"color", v.Color, "Laranja"},
		{"type", v.Type, "Motocicleta"},
		{"species", v.Species, "Passageiro"},
		{"category", v.Category, "Particular"},
		{"fuel", v.Fuel, "Gasolina"},
		{"city", v.City, "PRAIA GRANDE"},
		{"plateState", v.PlateState, "SC"},
		{"renavamStatus", v.RenavamStatus, "Em circulação"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q; want %q", c.field, c.got, c.want)
		}
	}
	if v.ManufactureYear != 2019 || v.ModelYear != 2019 {
		t.Errorf("years = %d/%d; want 2019/2019", v.ManufactureYear, v.ModelYear)
	}
	if v.OwnerName == "" {
		t.Error("ownerName empty; want the masked name")
	}
	// SC does not provide chassis or owner CPF.
	if v.Chassis != "" || v.OwnerCPF != "" {
		t.Errorf("expected no chassis/ownerCpf for SC, got chassis=%q ownerCpf=%q", v.Chassis, v.OwnerCPF)
	}

	// --- Licensing ---
	if resp.Licensing == nil || resp.Licensing.Year != "2026" {
		t.Errorf("licensing = %+v; want year 2026", resp.Licensing)
	}

	// --- Fines: 2 itemized (from historicoInfracoes; infracoes empty) ---
	if len(resp.Fines) != 2 {
		t.Fatalf("expected 2 fines, got %d: %+v", len(resp.Fines), resp.Fines)
	}
	byNotice := map[string]model.Fine{}
	for _, f := range resp.Fines {
		byNotice[f.Notice] = f
	}
	if f := byNotice["8779D26488"]; f.Amount != "130.16" || f.Status != "Paga" {
		t.Errorf("fine 8779D26488 = %+v; want amount 130.16 / status Paga", f)
	}
	if f := byNotice["E029874942"]; f.Amount != "195.23" {
		t.Errorf("fine E029874942 amount = %q; want 195.23", f.Amount)
	}

	// --- Debts: debitos + dividaAtiva both empty ---
	if len(resp.Debts) != 0 {
		t.Errorf("expected 0 debts, got %d: %+v", len(resp.Debts), resp.Debts)
	}

	// --- Restrictions: sale restriction (alienação) + SNG pending ---
	types := map[string]bool{}
	for _, r := range resp.Restrictions {
		types[r.Type] = true
	}
	if !types["SALE_RESTRICTION"] {
		t.Error("expected SALE_RESTRICTION (restricaoVenda / alienação fiduciária)")
	}
	if !types["SNG_PENDING"] {
		t.Error("expected SNG_PENDING (informacoesPendentesSng)")
	}
}
