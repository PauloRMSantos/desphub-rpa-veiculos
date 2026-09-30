package model

import (
	"encoding/json"
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

type SessionCredentials struct {
	Bearer string `json:"bearer"`
	UserID string `json:"userId"`
}

type QueryRequest struct {
	Plate   string `json:"plate"`
	Renavam string `json:"renavam"`
	Chassis string `json:"chassis,omitempty"`
	State       string              `json:"state,omitempty"`
	Types       []QueryType         `json:"types"`
	Credentials *Credentials        `json:"credentials,omitempty"`
	Session     *SessionCredentials `json:"session,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Vehicle struct {
	Plate           string `json:"plate,omitempty"`
	PreviousPlate   string `json:"previousPlate,omitempty"`
	Renavam         string `json:"renavam,omitempty"`
	Chassis         string `json:"chassis,omitempty"`
	MakeModel       string `json:"makeModel,omitempty"`
	ManufactureYear int    `json:"manufactureYear,omitempty"`
	ModelYear       int    `json:"modelYear,omitempty"`
	Color           string `json:"color,omitempty"`
	Type            string `json:"type,omitempty"`
	Species         string `json:"species,omitempty"`
	Category        string `json:"category,omitempty"` // tem no detran sc, mas não no detran rs
	Fuel            string `json:"fuel,omitempty"`     // tem no detran sc, mas não no detran rs
	City            string `json:"city,omitempty"`
	PlateState      string `json:"plateState,omitempty"`
	RenavamStatus   string `json:"renavamStatus,omitempty"`
	OwnerName       string `json:"ownerName,omitempty"`
	OwnerCPF        string `json:"ownerCpf,omitempty"`
}

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
	Amount  string `json:"amount"`
	DueDate string `json:"dueDate,omitempty"`
}

type TaxEntry struct {
	Year       string `json:"year"`
	Status     string `json:"status"` 
	Amount     string `json:"amount"` 
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
	JobID                  string                  `json:"jobId"`
	Plate                  string                  `json:"plate"`
	Source                 string                  `json:"source"`
	CollectedAt            time.Time               `json:"collectedAt"`
	Vehicle                *Vehicle                `json:"vehicle,omitempty"`
	Licensing              *Licensing              `json:"licensing,omitempty"`
	Violations             *Violations             `json:"violations,omitempty"`
	Restrictions           []Restriction           `json:"restrictions,omitempty"`
	SpecialCharacteristics []SpecialCharacteristic `json:"specialCharacteristics,omitempty"`
	Taxes                  []TaxEntry              `json:"taxes,omitempty"`
	Debts                  []Debt                  `json:"debts,omitempty"`
	Fines                  []Fine                  `json:"fines,omitempty"`
	Status                 Status                  `json:"status"`
	Errors                 []StepError             `json:"errors,omitempty"`
}

type Fine struct {
	Notice      string `json:"notice,omitempty"`
	Description string `json:"description,omitempty"`
	Date        string `json:"date,omitempty"`
	Location    string `json:"location,omitempty"`
	Amount      string `json:"amount,omitempty"`
	Situation   string `json:"situation,omitempty"`
	Status      string `json:"status,omitempty"`
}

type SpecialCharacteristic struct {
	Description string `json:"description"`
	Origin      string `json:"origin,omitempty"`
	Code        string `json:"code,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
}
