# DespHub — Módulo RPA

Microsserviço em **Go + chromedp** que automatiza consultas nos portais
governamentais (DETRAN-RS via login gov.br) e entrega dados veiculares
normalizados ao backend Spring Boot do DespHub.

> Documento de contexto/escopo completo: `RPA.md` (fora do repo, fornecido pelo autor).

## Status

- **Fase 0 — Setup:** ✅ estrutura, config, logger, browser (chromedp), model, API skeleton.
- **Fase 1 — MVP:** ⏳ navigator (login gov.br + consulta) + parser (goquery) + endpoint síncrono. Aguardando URL exata e fixture HTML real.

## Arquitetura

Três camadas isoladas por interfaces:

- **navegação** (`internal/browser`, `internal/portals/*`): chromedp, frágil, muda com o portal.
- **parsing** (`internal/portals/*/parser.go`): goquery, testável com fixtures, **nunca toca rede**.
- **domínio** (`internal/model`, `internal/consulta`): structs limpas e casos de uso.

```
cmd/rpa            entrypoint (HTTP server)
internal/config    config via env (12-factor)
internal/logger    slog JSON + jobId de correlação
internal/browser   fábrica de sessão chromedp (timeout, headless, reuso de sessão)
internal/model     DTOs de request/response
internal/api       handlers REST
internal/portals   navegação + parsing por portal (detranrs)
internal/consulta  orquestração de domínio
internal/captcha   interface Solver (fase 4)
fixtures           HTML real dos portais para testes de parser
docker/Dockerfile  build multi-stage com Chromium
```

## Rodando local

```bash
cp .env.example .env      # ajuste conforme necessário
go run ./cmd/rpa
```

Health check:

```bash
curl -s localhost:8080/health
```

Contrato (ainda retorna 501 até a Fase 1):

```bash
curl -s -X POST localhost:8080/api/detran/consultas \
  -H 'content-type: application/json' \
  -d '{"placa":"ABC1D23","renavam":"00123456789","tipos":["DADOS_CADASTRAIS"]}'
```

## Testes

```bash
go test ./...                          # unitários (parsers com fixtures)
go test -tags e2e ./internal/browser   # smoke real do chromedp (abre o portal)
```

## Docker

```bash
docker build -f docker/Dockerfile -t desphub-rpa .
docker run --rm -p 8080:8080 desphub-rpa
```

## Notas LGPD

- Credenciais gov.br nunca são logadas nem retornadas.
- Placa é mascarada nos logs.
- `.env` e sessões persistidas ficam no `.gitignore`.
# desphub-rpa-veiculos
