package model

import (
	"errors"
	"time"
)

var ErrReconexaoNecessaria = errors.New("sessão do portal expirada — reconecte o gov.br")

type TipoConsulta string

const (
	TipoDadosCadastrais TipoConsulta = "DADOS_CADASTRAIS"
	TipoDebitos         TipoConsulta = "DEBITOS"
	TipoRestricoes      TipoConsulta = "RESTRICOES"
	TipoLicenciamento   TipoConsulta = "LICENCIAMENTO"
)

type Status string

const (
	StatusSucesso Status = "SUCESSO"
	StatusParcial Status = "PARCIAL"
	StatusErro    Status = "ERRO"
)

type Credenciais struct {
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
}

type ConsultaRequest struct {
	Placa       string         `json:"placa"`
	Renavam     string         `json:"renavam"`
	Chassi      string         `json:"chassi,omitempty"`
	Tipos       []TipoConsulta `json:"tipos"`
	Credenciais *Credenciais   `json:"credenciais,omitempty"`
}

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
	CpfProprietario string `json:"cpfProprietario,omitempty"`
}

type Licenciamento struct {
	Exercicio         string `json:"exercicio,omitempty"`
	SituacaoDocumento string `json:"situacaoDocumento,omitempty"`
	Documento         string `json:"documento,omitempty"` // ex.: "CRLV"
	DataVencimento    string `json:"dataVencimento,omitempty"`
}

type Restricao struct {
	Tipo      string `json:"tipo"`
	Descricao string `json:"descricao"`
}

type Debito struct {
	Tipo       string `json:"tipo"`
	Exercicio  int    `json:"exercicio,omitempty"`
	Valor      string `json:"valor"` // decimal como string, ex.: "1234.56"
	Vencimento string `json:"vencimento,omitempty"`
}

type ResumoInfracao struct {
	Quantidade int    `json:"quantidade"`
	Valor      string `json:"valor"`
}

type Infracoes struct {
	AVencer               ResumoInfracao `json:"aVencer"`
	Vencidas              ResumoInfracao `json:"vencidas"`
	Suspensas             ResumoInfracao `json:"suspensas"`
	AguardandoPrazoDefesa ResumoInfracao `json:"aguardandoPrazoDefesa"`
	AguardandoJulgamento  ResumoInfracao `json:"aguardandoJulgamento"`
}

type EtapaErro struct {
	Etapa    string `json:"etapa"`
	Mensagem string `json:"mensagem"`
}

type ConsultaResponse struct {
	JobID      string      `json:"jobId"`
	Placa      string      `json:"placa"`
	Fonte      string      `json:"fonte"`
	ColetadoEm time.Time   `json:"coletadoEm"`
	Veiculo       *Veiculo       `json:"veiculo,omitempty"`
	Licenciamento *Licenciamento `json:"licenciamento,omitempty"`
	Infracoes     *Infracoes     `json:"infracoes,omitempty"`
	Restricoes    []Restricao    `json:"restricoes,omitempty"`
	Debitos       []Debito       `json:"debitos,omitempty"`
	Status        Status         `json:"status"`
	Erros         []EtapaErro    `json:"erros,omitempty"`
}
