// Package model define os contratos (DTOs) de entrada e saída do serviço,
// alinhados com o backend Spring Boot.
//
// Convenção crítica: valores monetários são SEMPRE string decimal (ex.: "1234.56")
// para casar com BigDecimal no backend. Nunca float.
package model

import "time"

// TipoConsulta enumera os grupos de dados que podem ser solicitados.
type TipoConsulta string

const (
	TipoDadosCadastrais TipoConsulta = "DADOS_CADASTRAIS"
	TipoDebitos         TipoConsulta = "DEBITOS"
	TipoRestricoes      TipoConsulta = "RESTRICOES"
	TipoLicenciamento   TipoConsulta = "LICENCIAMENTO"
)

// Status representa o resultado geral de uma consulta.
type Status string

const (
	StatusSucesso Status = "SUCESSO"
	StatusParcial Status = "PARCIAL"
	StatusErro    Status = "ERRO"
)

// Credenciais são as credenciais do portal (gov.br), quando exigido login.
// NUNCA devem ser logadas nem serializadas em respostas.
type Credenciais struct {
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
}

// ConsultaRequest é o corpo de POST /api/v1/consultas.
type ConsultaRequest struct {
	Placa       string         `json:"placa"`
	Renavam     string         `json:"renavam"`
	Chassi      string         `json:"chassi,omitempty"`
	Tipos       []TipoConsulta `json:"tipos"`
	Credenciais *Credenciais   `json:"credenciais,omitempty"`
}

// Veiculo agrupa os dados cadastrais normalizados.
type Veiculo struct {
	Placa           string `json:"placa,omitempty"`
	Renavam         string `json:"renavam,omitempty"`
	Chassi          string `json:"chassi,omitempty"`
	MarcaModelo     string `json:"marcaModelo,omitempty"`
	AnoFabricacao   int    `json:"anoFabricacao,omitempty"`
	AnoModelo       int    `json:"anoModelo,omitempty"`
	Cor             string `json:"cor,omitempty"`
	Tipo            string `json:"tipo,omitempty"`     // ex.: "Automóvel"
	Especie         string `json:"especie,omitempty"`  // ex.: "Passageiro"
	Categoria       string `json:"categoria,omitempty"`
	Municipio       string `json:"municipio,omitempty"` // município de registro
	UfPlaca         string `json:"ufPlaca,omitempty"`
	Combustivel     string `json:"combustivel,omitempty"`
	SituacaoRenavam string `json:"situacaoRenavam,omitempty"` // ex.: "Em circulação"
	// CpfProprietario é PII sensível: NUNCA logar em claro; entregue só ao backend.
	CpfProprietario string `json:"cpfProprietario,omitempty"`
}

// Licenciamento resume a situação do CRLV/exercício atual.
type Licenciamento struct {
	Exercicio         string `json:"exercicio,omitempty"`
	SituacaoDocumento string `json:"situacaoDocumento,omitempty"`
	Documento         string `json:"documento,omitempty"` // ex.: "CRLV"
	DataVencimento    string `json:"dataVencimento,omitempty"`
}

// Restricao descreve um bloqueio/restrição administrativa ou judicial.
type Restricao struct {
	Tipo      string `json:"tipo"`
	Descricao string `json:"descricao"`
}

// Debito descreve um débito (IPVA, licenciamento, taxa, multa).
// Valor é string decimal (casar com BigDecimal).
type Debito struct {
	Tipo       string `json:"tipo"`
	Exercicio  int    `json:"exercicio,omitempty"`
	Valor      string `json:"valor"` // decimal como string, ex.: "1234.56"
	Vencimento string `json:"vencimento,omitempty"`
}

// EtapaErro registra em que passo do fluxo houve falha.
type EtapaErro struct {
	Etapa    string `json:"etapa"`
	Mensagem string `json:"mensagem"`
}

// ConsultaResponse é a resposta de POST /api/v1/consultas.
type ConsultaResponse struct {
	JobID      string      `json:"jobId"`
	Placa      string      `json:"placa"`
	Fonte      string      `json:"fonte"`
	ColetadoEm time.Time   `json:"coletadoEm"`
	Veiculo       *Veiculo       `json:"veiculo,omitempty"`
	Licenciamento *Licenciamento `json:"licenciamento,omitempty"`
	Restricoes    []Restricao    `json:"restricoes,omitempty"`
	Debitos       []Debito       `json:"debitos,omitempty"`
	Status        Status         `json:"status"`
	Erros         []EtapaErro    `json:"erros,omitempty"`
}
