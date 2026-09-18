# DespHub RPA — Vehicle Query (DETRAN-RS)

Go + chromedp microservice that automates queries on government portals
(DETRAN-RS via gov.br login) and delivers normalized vehicle data to the DespHub
Spring Boot backend.

> Integration guide for back/front: [docs/INTEGRATION.md](docs/INTEGRATION.md).

## Architecture

Three layers isolated by interfaces:

- **navigation** (`internal/browser`, `internal/portals/*`): chromedp, fragile,
  changes with the portal.
- **parsing** (`internal/portals/*/parser.go`): JSON → normalized contract,
  testable with fixtures, **never touches the network**.
- **domain** (`internal/model`, `internal/query`): clean structs and use cases.

```
cmd/rpa            entrypoint (HTTP server)
cmd/desphub-login  Mac helper: captures the gov.br token and pushes it (token mode)
internal/config    env config (12-factor)
internal/logger    slog JSON + jobId correlation
internal/browser   chromedp session factory (local and remote/attach)
internal/model     request/response DTOs (the public contract)
internal/query     domain orchestration (status, anonymization)
internal/api       REST handlers
internal/portals   navigation + parsing per portal (detranrs)
internal/pii       PII anonymization (LGPD)
fixtures           real portal JSON for parser tests
deploy/            systemd unit + env for VPS deploy
docs/              integration guide
```

## Login modes

The gov.br login needs a real browser + occasional human. Two modes:

- **`LOGIN_MODE=manual`** (local/dev): the RPA attaches to a Chrome you open with
  remote debugging (`scripts/start-chrome.sh`) and captures the token.
- **`LOGIN_MODE=token`** (VPS/prod): the RPA runs headless with no browser; the
  token is captured on your Mac and pushed via `POST /api/detran/session/token`.

## Running locally (manual mode)

```bash
bash scripts/start-chrome.sh          # opens the dedicated Chrome
LOGIN_MODE=manual PORT=8090 go run ./cmd/rpa
```

Health check:

```bash
curl -s localhost:8090/health
```

Query:

```bash
curl -s -X POST localhost:8090/api/detran/queries \
  -H 'content-type: application/json' \
  -d '{"plate":"IDX1756","renavam":"00561040575","types":["REGISTRATION","DEBTS","RESTRICTIONS","LICENSING"]}'
```

## Tests

```bash
go test ./...                          # unit (parsers with fixtures)
go test -tags e2e ./internal/browser   # real chromedp smoke (opens the portal)
```

## Deploy on a VPS (no Docker, no Go on the host)

Cross-compile on the Mac, copy the binary, run as a systemd service:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o desphub-rpa ./cmd/rpa
scp desphub-rpa deploy/desphub-rpa.* user@vps:/tmp/
```

See [deploy/desphub-rpa.service](deploy/desphub-rpa.service) for the install steps.

## LGPD notes

- gov.br credentials/token are never logged nor returned.
- The plate is masked in logs; the owner CPF is masked in the response
  (`ANONYMIZE_PII=true`).
- `.env` and persisted sessions are in `.gitignore`.
