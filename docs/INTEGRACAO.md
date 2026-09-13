# Guia de Integração — DespHub RPA (Consulta Veicular DETRAN-RS)

Guia para integrar o microsserviço RPA no **backend (Spring Boot)** e, por tabela,
no **frontend** do DespHub.

> TL;DR: o **frontend fala com o backend do DespHub**; o **backend fala com o RPA**
> via um único endpoint REST (`POST /api/detran/consultas`). O RPA devolve os dados
> veiculares já **normalizados**. O front **nunca** chama o RPA direto.

---

## 1. Arquitetura em uma imagem

```
┌───────────┐        ┌────────────────────┐      ┌──────────────────────┐     ┌──────────────┐
│ Frontend  │ ─────▶ │ Backend DespHub     │ ───▶ │ RPA (este serviço)   │ ──▶ │ API DETRAN-RS│
│ (browser) │  HTTP  │ (Spring Boot)       │ REST │ /api/detran/consultas│ JWT │ (Procergs)   │
└───────────┘        └────────────────────┘      └──────────────────────┘     └──────────────┘
       ▲                      │                            │
       │  "Reconectar gov.br" │                            │ o token gov.br é injetado
       └──────────────────────┘                            │ de fora (ver seção 6)
```

- **Front → Backend:** o front chama endpoints do **seu** backend (autenticados pelo
  login do DespHub). Nunca chama o RPA diretamente (rede interna, sem CORS público).
- **Backend → RPA:** o backend chama o RPA por HTTP na rede interna/privada.
- **RPA → DETRAN:** o RPA usa um token de sessão gov.br (obtido fora, ver seção 6).

---

## 2. Referência de endpoints

Base URL do RPA (exemplo): `http://rpa.interno:8080` (nunca exponha publicamente).

| Método | Rota | Para quê |
|--------|------|----------|
| `GET`  | `/health` | Liveness/readiness (retorna `{"status":"ok"}`) |
| `POST` | `/api/detran/consultas` | **Consulta um veículo** (o que o backend chama) |
| `GET`  | `/api/detran/sessao` | Estado da sessão (autenticado? quando expira?) |
| `POST` | `/api/detran/sessao/token` | Injeta o token (uso operacional, ver seção 6) |
| `POST` | `/api/detran/sessao/reconectar` | Dispara re-login manual (só no modo `manual`) |

### 2.1 `POST /api/detran/consultas`

**Request:**
```json
{
  "placa": "IDX1756",
  "renavam": "00561040575",
  "tipos": ["DADOS_CADASTRAIS", "DEBITOS", "RESTRICOES", "LICENCIAMENTO"]
}
```
- `placa` **ou** `renavam` obrigatório (recomendado enviar os dois).
- `tipos`: lista com ao menos um de `DADOS_CADASTRAIS`, `DEBITOS`, `RESTRICOES`,
  `LICENCIAMENTO`. (Hoje a API do DETRAN devolve tudo numa chamada; os `tipos`
  documentam a intenção e permitem filtragem futura.)

**Response (200):** ver o contrato completo na seção 3.

### 2.2 Códigos de status (o backend DEVE tratar)

| HTTP | Significado | O que o backend faz |
|------|-------------|---------------------|
| `200` | Sucesso (`status: SUCESSO` ou `PARCIAL`) | Usa os dados. `PARCIAL` = veio incompleto, trate como aviso |
| `422` | Validação (faltou placa/renavam ou tipos) | Erro do chamador; corrija o request |
| `400` | JSON inválido | Bug de integração |
| `502` | Falha ao falar com o portal DETRAN | Transitório — pode retentar com backoff |
| `503` | **Sessão gov.br expirada** (`etapa: reconexao`) | Sinalize "reconecte o gov.br"; não adianta retentar até renovar o token |
| `501` | RPA sem login configurado | Erro de deploy (LOGIN_MODE não setado) |

> Mesmo em erro, o corpo é um `ConsultaResponse` com `status: "ERRO"` e a lista
> `erros[]` explicando a `etapa`.

---

## 3. Contrato de dados (response)

```json
{
  "jobId": "uuid-de-correlação",
  "placa": "IDX1756",
  "fonte": "DETRAN-RS",
  "coletadoEm": "2026-09-06T12:00:00Z",
  "veiculo": {
    "placa": "IDX1756",
    "renavam": "561040575",
    "chassi": "9BWZZZ...",
    "marcaModelo": "VW/FUSCA",
    "anoFabricacao": 1986,
    "anoModelo": 1986,
    "cor": "Bege",
    "tipo": "Automóvel",
    "especie": "Passageiro",
    "categoria": "",
    "municipio": "CARAA",
    "ufPlaca": "RS",
    "combustivel": "",
    "situacaoRenavam": "Em circulação",
    "cpfProprietario": "***.223.100-**"
  },
  "licenciamento": {
    "exercicio": "2026",
    "situacaoDocumento": "Documento disponibilizado",
    "documento": "CRLV",
    "dataVencimento": "31/07/2027"
  },
  "infracoes": {
    "aVencer":               { "quantidade": 1, "valor": "130.16" },
    "vencidas":              { "quantidade": 2, "valor": "293.47" },
    "suspensas":             { "quantidade": 0, "valor": "0.00" },
    "aguardandoPrazoDefesa": { "quantidade": 0, "valor": "0.00" },
    "aguardandoJulgamento":  { "quantidade": 1, "valor": "195.23" }
  },
  "restricoes": [
    { "tipo": "ADMINISTRATIVA", "descricao": "Restrição administrativa - RENAJUD" },
    { "tipo": "ROUBO_FURTO",    "descricao": "Veículo com registro de roubo/furto" }
  ],
  "debitos": [
    { "tipo": "IPVA",  "exercicio": 2026, "valor": "1234.56", "vencimento": "31/03/2026" },
    { "tipo": "DPVAT", "exercicio": 2026, "valor": "105.65" }
  ],
  "status": "SUCESSO",
  "erros": []
}
```

**Regras importantes:**
- **Valores monetários são `string` decimal** (ex.: `"1234.56"`) — mapeie para
  `BigDecimal` no Java, **nunca** `double`/`float`.
- Campos opcionais podem vir ausentes/vazios (`omitempty`). `veiculo`,
  `licenciamento`, `infracoes` podem ser `null`; `debitos`/`restricoes` podem faltar.
- `cpfProprietario` vem **mascarado** quando `ANONIMIZAR_PII=true` (padrão). Em
  produção você decide (ver seção 7).
- `status`: `SUCESSO` (dados completos), `PARCIAL` (faltou algo), `ERRO`.

---

## 4. Integração no backend (Spring Boot)

### 4.1 DTOs (records)

```java
public record ConsultaRequest(String placa, String renavam, List<String> tipos) {}

public record ConsultaResponse(
    String jobId, String placa, String fonte, Instant coletadoEm,
    Veiculo veiculo, Licenciamento licenciamento, Infracoes infracoes,
    List<Restricao> restricoes, List<Debito> debitos,
    String status, List<EtapaErro> erros
) {}

public record Veiculo(
    String placa, String renavam, String chassi, String marcaModelo,
    Integer anoFabricacao, Integer anoModelo, String cor, String tipo,
    String especie, String categoria, String municipio, String ufPlaca,
    String combustivel, String situacaoRenavam, String cpfProprietario
) {}

public record Licenciamento(String exercicio, String situacaoDocumento,
                            String documento, String dataVencimento) {}

public record ResumoInfracao(int quantidade, BigDecimal valor) {}
public record Infracoes(ResumoInfracao aVencer, ResumoInfracao vencidas,
                        ResumoInfracao suspensas, ResumoInfracao aguardandoPrazoDefesa,
                        ResumoInfracao aguardandoJulgamento) {}

public record Restricao(String tipo, String descricao) {}
public record Debito(String tipo, Integer exercicio, BigDecimal valor, String vencimento) {}
public record EtapaErro(String etapa, String mensagem) {}
```
> `valor` como `BigDecimal`: o Jackson converte a string `"1234.56"` em `BigDecimal`
> automaticamente. Garanta que **não** está usando `double`.

### 4.2 Client (WebClient)

```java
@Service
public class RpaDetranClient {
    private final WebClient http;

    public RpaDetranClient(@Value("${rpa.base-url}") String baseUrl) {
        this.http = WebClient.builder().baseUrl(baseUrl).build();
    }

    public ConsultaResponse consultar(String placa, String renavam) {
        return http.post()
            .uri("/api/detran/consultas")
            .bodyValue(new ConsultaRequest(placa, renavam,
                List.of("DADOS_CADASTRAIS","DEBITOS","RESTRICOES","LICENCIAMENTO")))
            .retrieve()
            .onStatus(s -> s.value() == 503, r ->
                Mono.error(new SessaoExpiradaException("Reconecte o gov.br")))
            .onStatus(HttpStatusCode::isError, r ->
                r.bodyToMono(ConsultaResponse.class).map(PortalException::new))
            .bodyToMono(ConsultaResponse.class)
            .timeout(Duration.ofSeconds(60))
            .block();
    }
}
```
Config: `rpa.base-url=http://rpa.interno:8080`.

### 4.3 Tratamento de erros recomendado

- **503 (`etapa: reconexao`)** → devolva ao front um estado do tipo
  "integração DETRAN desconectada, reconecte" (não é erro do usuário).
- **502** → transitório; pode ter 1 retry com backoff (o RPA já retenta a rede
  internamente 3x, então um 502 costuma ser real indisponibilidade do portal).
- **422** → problema no que o front mandou (placa/renavam ausentes).

---

## 5. Integração no frontend

O front **não chama o RPA**. Ele chama um endpoint do **seu backend**, por exemplo
`GET /veiculos/{placa}?renavam=...`, que por baixo usa o `RpaDetranClient`.

Fluxo sugerido:
1. Usuário digita placa + RENAVAM na tela do DespHub.
2. Front → `GET /api/veiculos/consulta?placa=...&renavam=...` (seu backend).
3. Backend chama o RPA, recebe o `ConsultaResponse`, aplica regras de negócio
   (ex.: salvar histórico, checar permissão do despachante) e devolve ao front.
4. Front renderiza: dados do veículo, débitos (IPVA/DPVAT), infrações, restrições.

**UX de sessão:** se o backend receber `503` do RPA, mostre um aviso tipo
"Conexão com o DETRAN expirou — reconecte" em vez de um erro genérico. Quem
reconecta é o **operador** (ver seção 6), não o usuário final.

---

## 6. Gestão da sessão/token (operacional)

O RPA precisa de um token de sessão gov.br para consultar. Como ele é obtido
depende do **modo** (`LOGIN_MODE`):

### Modo `token` (recomendado para VPS/produção)
- O RPA **não abre navegador**. O token é capturado no **Mac do operador** e
  empurrado para o RPA:
  ```bash
  # No Mac, com o Chrome sidecar aberto (scripts/start-chrome.sh):
  RPA_URL=http://rpa.interno:8080 SESSAO_TOKEN_APIKEY=<chave> go run ./cmd/desphub-login
  ```
- O endpoint `POST /api/detran/sessao/token` (header `X-Api-Key`) recebe o token.
- Consulte o estado a qualquer momento:
  ```bash
  curl http://rpa.interno:8080/api/detran/sessao
  # {"autenticado":true,"expiraEm":"...","expiraEmSegundos":1799,"reportaStatus":true}
  ```
- Quando `expiraEmSegundos` chega perto de zero (ou uma consulta volta `503`),
  rode o helper de novo. Dá para **automatizar** com um agendador no Mac.

### Modo `manual` (uso local/desenvolvimento)
- O RPA anexa a um Chrome real e o operador loga na janela.
- `POST /api/detran/sessao/reconectar` força um novo login humano.

> **O backend não precisa saber de token.** Isso é responsabilidade operacional do
> RPA/operador. O backend só chama `/consultas` e trata o `503` quando a sessão cai.

---

## 7. Configuração (variáveis de ambiente)

| Var | Default | Descrição |
|-----|---------|-----------|
| `PORT` | `8080` | Porta HTTP |
| `LOGIN_MODE` | *(vazio)* | `token` (VPS) ou `manual` (local). Vazio = consultas dão 501 |
| `SESSAO_TOKEN_APIKEY` | *(vazio)* | Protege `POST /sessao/token` (obrigatório em `token`) |
| `CHROME_DEBUG_URL` | `http://localhost:9222` | Chrome de depuração (modo `manual`) |
| `ANONIMIZAR_PII` | `true` | `false` = CPF do proprietário sem máscara (produção) |
| `DETRANRS_URL` | *(default interno)* | Override do host da API DETRAN |
| `LOG_LEVEL` | `info` | `debug` para diagnóstico |

Deploy sem Docker/Go na VPS: cross-compile no Mac + systemd — ver `deploy/`.

---

## 8. Segurança

- **Nunca exponha o RPA na internet pública.** Deixe-o na rede interna; só o
  backend do DespHub o acessa. Se precisar de acesso externo, ponha atrás de
  Nginx/Caddy com TLS + IP allowlist.
- `SESSAO_TOKEN_APIKEY` protege a injeção de token — gere com `openssl rand -hex 32`.
- O token gov.br e o CPF do proprietário são **dados sensíveis**: não logue,
  mantenha `ANONIMIZAR_PII` conforme a política de LGPD do DespHub.

---

## 9. Exemplos rápidos (curl)

```bash
# Saúde
curl http://localhost:8080/health

# Consulta
curl -s -X POST http://localhost:8080/api/detran/consultas \
  -H 'content-type: application/json' \
  -d '{"placa":"IDX1756","renavam":"00561040575","tipos":["DADOS_CADASTRAIS","DEBITOS","RESTRICOES","LICENCIAMENTO"]}' | python3 -m json.tool

# Estado da sessão
curl http://localhost:8080/api/detran/sessao
```
