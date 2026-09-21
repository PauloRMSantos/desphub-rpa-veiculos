package query

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

// fakePortal implements Portal without touching the network.
type fakePortal struct {
	resp *model.QueryResponse
	err  error
}

func (p fakePortal) Name() string { return "FAKE" }
func (p fakePortal) Query(_ context.Context, _, _ string, _ *model.SessionCredentials) (*model.QueryResponse, error) {
	return p.resp, p.err
}

func newLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// reconnectablePortal implements Portal + Reconnectable.
type reconnectablePortal struct {
	fakePortal
	reconnectCalled bool
	reconnectErr    error
}

func (p *reconnectablePortal) Reconnect(_ context.Context) error {
	p.reconnectCalled = true
	return p.reconnectErr
}

func TestExecuteSuccess(t *testing.T) {
	fake := fakePortal{resp: &model.QueryResponse{Vehicle: &model.Vehicle{MakeModel: "VW/FUSCA"}}}
	svc := NewService(fake, newLog(), false)

	got := svc.Execute(context.Background(), "job-1", model.QueryRequest{Plate: "ABC1D23"})

	if got.Status != model.StatusSuccess {
		t.Errorf("status = %q; want SUCCESS", got.Status)
	}
	if got.JobID != "job-1" {
		t.Errorf("jobId = %q; want job-1", got.JobID)
	}
	if got.Source != "FAKE" {
		t.Errorf("source = %q; want FAKE", got.Source)
	}
	if got.CollectedAt.IsZero() {
		t.Error("collectedAt should not be zero")
	}
}

func TestExecuteErrorBecomesStatusError(t *testing.T) {
	fake := fakePortal{err: errors.New("boom")}
	svc := NewService(fake, newLog(), false)

	got := svc.Execute(context.Background(), "job-2", model.QueryRequest{Plate: "ABC1D23"})

	if got.Status != model.StatusError {
		t.Fatalf("status = %q; want ERROR", got.Status)
	}
	if len(got.Errors) != 1 || got.Errors[0].Step != "portal" {
		t.Errorf("expected 1 error at step 'portal', got %+v", got.Errors)
	}
}

func TestExecuteReconnectRequiredMarksStep(t *testing.T) {
	fake := fakePortal{err: model.ErrReconnectRequired}
	svc := NewService(fake, newLog(), false)

	got := svc.Execute(context.Background(), "job-r", model.QueryRequest{Plate: "ABC1D23"})

	if got.Status != model.StatusError {
		t.Fatalf("status = %q; want ERROR", got.Status)
	}
	if len(got.Errors) != 1 || got.Errors[0].Step != "reconnect" {
		t.Errorf("expected step 'reconnect', got %+v", got.Errors)
	}
}

func TestExecuteWithoutVehicleBecomesPartial(t *testing.T) {
	fake := fakePortal{resp: &model.QueryResponse{}}
	svc := NewService(fake, newLog(), false)

	got := svc.Execute(context.Background(), "job-3", model.QueryRequest{Plate: "ABC1D23"})

	if got.Status != model.StatusPartial {
		t.Errorf("status = %q; want PARTIAL", got.Status)
	}
}

func TestAnonymizeOnMasksCPF(t *testing.T) {
	fake := fakePortal{resp: &model.QueryResponse{
		Vehicle: &model.Vehicle{MakeModel: "VW/FUSCA", OwnerCPF: "74722310025"},
	}}
	svc := NewService(fake, newLog(), true)

	got := svc.Execute(context.Background(), "job-4", model.QueryRequest{Plate: "ABC1D23"})

	if got.Vehicle.OwnerCPF != "***.223.100-**" {
		t.Errorf("CPF = %q; want masked ***.223.100-**", got.Vehicle.OwnerCPF)
	}
}

func TestAnonymizeOffKeepsCPF(t *testing.T) {
	fake := fakePortal{resp: &model.QueryResponse{
		Vehicle: &model.Vehicle{MakeModel: "VW/FUSCA", OwnerCPF: "74722310025"},
	}}
	svc := NewService(fake, newLog(), false)

	got := svc.Execute(context.Background(), "job-5", model.QueryRequest{Plate: "ABC1D23"})

	if got.Vehicle.OwnerCPF != "74722310025" {
		t.Errorf("CPF = %q; want unmasked", got.Vehicle.OwnerCPF)
	}
}

func TestReconnectCallsPortal(t *testing.T) {
	fake := &reconnectablePortal{}
	svc := NewService(fake, newLog(), false)

	if err := svc.Reconnect(context.Background()); err != nil {
		t.Fatalf("Reconnect error: %v", err)
	}
	if !fake.reconnectCalled {
		t.Error("expected portal.Reconnect to be called")
	}
}

func TestReconnectPortalUnsupported(t *testing.T) {
	svc := NewService(fakePortal{}, newLog(), false)
	if err := svc.Reconnect(context.Background()); err == nil {
		t.Error("expected error when portal does not support reconnection")
	}
}
