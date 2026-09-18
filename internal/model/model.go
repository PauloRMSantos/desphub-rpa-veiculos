package model

import (
	"errors"
	"time"
)

var ErrReconnectRequired = errors.New("portal session expired — reconnect gov.br")

type QueryType string

const (
	QueryRegistration QueryType = "REGISTRATION"
	QueryDebts        QueryType = "DEBTS"
	QueryRestrictions QueryType = "RESTRICTIONS"
	QueryLicensing    QueryType = "LICENSING"
)

type Status string

const (
	StatusSuccess Status = "SUCCESS"
	StatusPartial Status = "PARTIAL"
	StatusError   Status = "ERROR"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type QueryRequest struct {
	Plate       string       `json:"plate"`
	Renavam     string       `json:"renavam"`
	Chassis     string       `json:"chassis,omitempty"`
	Types       []QueryType  `json:"types"`
	Credentials *Credentials `json:"credentials,omitempty"`
}

type Vehicle struct {
	Plate           string `json:"plate,omitempty"`
	Renavam         string `json:"renavam,omitempty"`
	Chassis         string `json:"chassis,omitempty"`
	MakeModel       string `json:"makeModel,omitempty"`
	ManufactureYear int    `json:"manufactureYear,omitempty"`
	ModelYear       int    `json:"modelYear,omitempty"`
	Color           string `json:"color,omitempty"`
	Type            string `json:"type,omitempty"`    // e.g. "Automóvel" (portal value)
	Species         string `json:"species,omitempty"` // e.g. "Passageiro" (portal value)
	Category        string `json:"category,omitempty"`
	City            string `json:"city,omitempty"` 
	PlateState      string `json:"plateState,omitempty"`
	Fuel            string `json:"fuel,omitempty"`
	RenavamStatus   string `json:"renavamStatus,omitempty"`
	OwnerCPF string `json:"ownerCpf,omitempty"`
}

// Licensing summarizes the CRLV / current-year licensing status.
type Licensing struct {
	Year           string `json:"year,omitempty"`
	DocumentStatus string `json:"documentStatus,omitempty"`
	Document       string `json:"document,omitempty"` // e.g. "CRLV"
	DueDate        string `json:"dueDate,omitempty"`
}

type Restriction struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Debt struct {
	Type    string `json:"type"`
	Year    int    `json:"year,omitempty"`
	Amount  string `json:"amount"` // decimal string, e.g. "1234.56"
	DueDate string `json:"dueDate,omitempty"`
}

// TaxEntry is one year of the IPVA history (vehicle property tax), preserving
// the portal's status ("Isento", "Liquidado", "Devido", ...) — not only owed years.
type TaxEntry struct {
	Year       string `json:"year"`
	Status     string `json:"status"` // portal value: Isento / Liquidado / Devido / ...
	Amount     string `json:"amount"` // decimal string
	DueDate    string `json:"dueDate,omitempty"`
	ActiveDebt bool   `json:"activeDebt"`
}

type ViolationSummary struct {
	Count  int    `json:"count"`
	Amount string `json:"amount"`
}

type Violations struct {
	Upcoming         ViolationSummary `json:"upcoming"`
	Overdue          ViolationSummary `json:"overdue"`
	Suspended        ViolationSummary `json:"suspended"`
	AwaitingDefense  ViolationSummary `json:"awaitingDefense"`
	AwaitingJudgment ViolationSummary `json:"awaitingJudgment"`
}

type StepError struct {
	Step    string `json:"step"`
	Message string `json:"message"`
}

type QueryResponse struct {
	JobID       string       `json:"jobId"`
	Plate       string       `json:"plate"`
	Source      string       `json:"source"`
	CollectedAt time.Time    `json:"collectedAt"`
	Vehicle     *Vehicle     `json:"vehicle,omitempty"`
	Licensing   *Licensing   `json:"licensing,omitempty"`
	Violations  *Violations  `json:"violations,omitempty"`
	Restrictions []Restriction `json:"restrictions,omitempty"`
	Taxes       []TaxEntry   `json:"taxes,omitempty"` // full IPVA history (all years + status)
	Debts       []Debt       `json:"debts,omitempty"`
	Status      Status       `json:"status"`
	Errors      []StepError  `json:"errors,omitempty"`
}
