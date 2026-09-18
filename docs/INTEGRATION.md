# Integration Guide — DespHub RPA (DETRAN-RS Vehicle Query)

Guide to integrate the RPA microservice into the **backend (Spring Boot)** and,
by extension, the **frontend** of DespHub.

> TL;DR: the **frontend talks to the DespHub backend**; the **backend talks to
> the RPA** via a single REST endpoint (`POST /api/detran/queries`). The RPA
> returns already-normalized vehicle data. The frontend **never** calls the RPA
> directly.

---

## 1. Architecture in one picture

```
┌───────────┐        ┌────────────────────┐      ┌──────────────────────┐     ┌──────────────┐
│ Frontend  │ ─────▶ │ DespHub Backend     │ ───▶ │ RPA (this service)   │ ──▶ │ DETRAN-RS API│
│ (browser) │  HTTP  │ (Spring Boot)       │ REST │ /api/detran/queries  │ JWT │ (Procergs)   │
└───────────┘        └────────────────────┘      └──────────────────────┘     └──────────────┘
       ▲                      │                            │
       │   "Reconnect gov.br" │                            │ the gov.br token is injected
       └──────────────────────┘                            │ from outside (see section 6)
```

- **Front → Backend:** the front calls **your** backend endpoints (authenticated
  by DespHub's own login). It never calls the RPA directly (internal network, no
  public CORS).
- **Backend → RPA:** the backend calls the RPA over HTTP on the internal network.
- **RPA → DETRAN:** the RPA uses a gov.br session token (obtained externally, see §6).

---

## 2. Endpoint reference

RPA base URL (example): `http://rpa.internal:8080` (never expose it publicly).

| Method | Route | Purpose |
|--------|-------|---------|
| `GET`  | `/health` | Liveness/readiness (returns `{"status":"ok"}`) |
| `POST` | `/api/detran/queries` | **Query a vehicle** (what the backend calls) |
| `GET`  | `/api/detran/session` | Session state (authenticated? when does it expire?) |
| `POST` | `/api/detran/session/token` | Inject the token (operational, see §6) |
| `POST` | `/api/detran/session/reconnect` | Trigger a manual re-login (manual mode only) |

### 2.1 `POST /api/detran/queries`

**Request:**
```json
{
  "plate": "IDX1756",
  "renavam": "00561040575",
  "types": ["REGISTRATION", "DEBTS", "RESTRICTIONS", "LICENSING"]
}
```
- `plate` **or** `renavam` is required (sending both is recommended).
- `types`: a list with at least one of `REGISTRATION`, `DEBTS`, `RESTRICTIONS`,
  `LICENSING`. (Today the DETRAN API returns everything in one call; `types`
  documents intent and enables future filtering.)

**Response (200):** see the full contract in section 3.

### 2.2 Status codes (the backend MUST handle)

| HTTP | Meaning | What the backend does |
|------|---------|-----------------------|
| `200` | Success (`status: SUCCESS` or `PARTIAL`) | Use the data. `PARTIAL` = incomplete, treat as a warning |
| `422` | Validation (missing plate/renavam or types) | Caller error; fix the request |
| `400` | Invalid JSON | Integration bug |
| `502` | Failure talking to the DETRAN portal | Transient — may retry with backoff |
| `503` | **gov.br session expired** (`step: reconnect`) | Signal "reconnect gov.br"; don't retry until the token is refreshed |
| `501` | RPA login not configured | Deploy error (LOGIN_MODE not set) |

> Even on error, the body is a `QueryResponse` with `status: "ERROR"` and the
> `errors[]` list explaining the `step`.

---

## 3. Data contract (response)

```json
{
  "jobId": "correlation-uuid",
  "plate": "IDX1756",
  "source": "DETRAN-RS",
  "collectedAt": "2026-09-13T12:00:00Z",
  "vehicle": {
    "plate": "IDX1756",
    "renavam": "561040575",
    "chassis": "9BWZZZ...",
    "makeModel": "VW/FUSCA",
    "manufactureYear": 1986,
    "modelYear": 1986,
    "color": "Bege",
    "type": "Automóvel",
    "species": "Passageiro",
    "category": "",
    "city": "CARAA",
    "plateState": "RS",
    "fuel": "",
    "renavamStatus": "Em circulação",
    "ownerCpf": "***.223.100-**"
  },
  "licensing": {
    "year": "2026",
    "documentStatus": "Documento disponibilizado",
    "document": "CRLV",
    "dueDate": "31/07/2027"
  },
  "violations": {
    "upcoming":         { "count": 1, "amount": "130.16" },
    "overdue":          { "count": 2, "amount": "293.47" },
    "suspended":        { "count": 0, "amount": "0.00" },
    "awaitingDefense":  { "count": 0, "amount": "0.00" },
    "awaitingJudgment": { "count": 1, "amount": "195.23" }
  },
  "restrictions": [
    { "type": "ADMINISTRATIVA",  "description": "Restrição administrativa - RENAJUD" },
    { "type": "THEFT_ROBBERY",   "description": "Vehicle reported stolen/robbed" }
  ],
  "taxes": [
    { "year": "2026", "status": "Devido",    "amount": "1234.56", "dueDate": "31/03/2026", "activeDebt": false },
    { "year": "2025", "status": "Isento",    "amount": "0.00",    "activeDebt": false },
    { "year": "2023", "status": "Liquidado", "amount": "798.05",  "dueDate": "27/04/2023", "activeDebt": false }
  ],
  "debts": [
    { "type": "IPVA",  "year": 2026, "amount": "1234.56", "dueDate": "31/03/2026" },
    { "type": "DPVAT", "year": 2026, "amount": "105.65" }
  ],
  "status": "SUCCESS",
  "errors": []
}
```

**Important rules:**
- **Monetary values are decimal `string`s** (e.g. `"1234.56"`) — map to
  `BigDecimal` in Java, **never** `double`/`float`.
- **Field names are English; data values may be Portuguese** because they come
  straight from the DETRAN portal (e.g. `color: "Bege"`, `type: "Automóvel"`,
  `renavamStatus: "Em circulação"`). Do not expect these values to be translated.
- Optional fields may be absent/empty (`omitempty`). `vehicle`, `licensing`,
  `violations` may be `null`; `debts`/`restrictions` may be missing.
- `ownerCpf` is **masked** when `ANONYMIZE_PII=true` (default). You decide in
  production (see §7).
- `status`: `SUCCESS` (complete), `PARTIAL` (something missing), `ERROR`.
- Restriction `type` values we synthesize: `THEFT_ROBBERY`, `IMPOUNDED`. Values
  coming from the portal's own restriction list keep the portal's wording.
- **`taxes` vs `debts`**: `taxes` is the **full IPVA history** — every year with its
  portal `status` (`Isento`, `Liquidado`, `Devido`, ...), even paid/exempt years.
  `debts` is the **actionable** list (only what is actually owed now: owed IPVA +
  owed DPVAT). A settled ("Liquidado") or exempt ("Isento") year appears in `taxes`
  but not in `debts`. `taxes[].status` values come from the portal (Portuguese).

---

## 4. Backend integration (Spring Boot)

### 4.1 DTOs (records)

```java
public record QueryRequest(String plate, String renavam, List<String> types) {}

public record QueryResponse(
    String jobId, String plate, String source, Instant collectedAt,
    Vehicle vehicle, Licensing licensing, Violations violations,
    List<Restriction> restrictions, List<TaxEntry> taxes, List<Debt> debts,
    String status, List<StepError> errors
) {}

public record TaxEntry(String year, String status, BigDecimal amount,
                       String dueDate, boolean activeDebt) {}

public record Vehicle(
    String plate, String renavam, String chassis, String makeModel,
    Integer manufactureYear, Integer modelYear, String color, String type,
    String species, String category, String city, String plateState,
    String fuel, String renavamStatus, String ownerCpf
) {}

public record Licensing(String year, String documentStatus,
                        String document, String dueDate) {}

public record ViolationSummary(int count, BigDecimal amount) {}
public record Violations(ViolationSummary upcoming, ViolationSummary overdue,
                         ViolationSummary suspended, ViolationSummary awaitingDefense,
                         ViolationSummary awaitingJudgment) {}

public record Restriction(String type, String description) {}
public record Debt(String type, Integer year, BigDecimal amount, String dueDate) {}
public record StepError(String step, String message) {}
```
> `amount` as `BigDecimal`: Jackson converts the `"1234.56"` string into
> `BigDecimal` automatically. Make sure you are **not** using `double`.

### 4.2 Client (WebClient)

```java
@Service
public class RpaDetranClient {
    private final WebClient http;

    public RpaDetranClient(@Value("${rpa.base-url}") String baseUrl) {
        this.http = WebClient.builder().baseUrl(baseUrl).build();
    }

    public QueryResponse query(String plate, String renavam) {
        return http.post()
            .uri("/api/detran/queries")
            .bodyValue(new QueryRequest(plate, renavam,
                List.of("REGISTRATION","DEBTS","RESTRICTIONS","LICENSING")))
            .retrieve()
            .onStatus(s -> s.value() == 503, r ->
                Mono.error(new SessionExpiredException("Reconnect gov.br")))
            .onStatus(HttpStatusCode::isError, r ->
                r.bodyToMono(QueryResponse.class).map(PortalException::new))
            .bodyToMono(QueryResponse.class)
            .timeout(Duration.ofSeconds(60))
            .block();
    }
}
```
Config: `rpa.base-url=http://rpa.internal:8080`.

### 4.3 Recommended error handling

- **503 (`step: reconnect`)** → surface to the front a state like "DETRAN
  integration disconnected, reconnect" (not a user error).
- **502** → transient; a single retry with backoff is fine (the RPA already
  retries the network 3x internally, so a 502 usually means real portal downtime).
- **422** → problem in what the front sent (missing plate/renavam).

---

## 5. Frontend integration

The front **does not call the RPA**. It calls a **backend** endpoint, e.g.
`GET /vehicles/{plate}?renavam=...`, which uses `RpaDetranClient` under the hood.

Suggested flow:
1. User types plate + RENAVAM in the DespHub screen.
2. Front → `GET /api/vehicles/query?plate=...&renavam=...` (your backend).
3. Backend calls the RPA, gets the `QueryResponse`, applies business rules (e.g.
   save history, check the broker's permission) and returns to the front.
4. Front renders: vehicle data, debts (IPVA/DPVAT), violations, restrictions.

**Session UX:** if the backend gets a `503` from the RPA, show something like
"DETRAN connection expired — reconnect" instead of a generic error. The person
who reconnects is the **operator** (see §6), not the end user.

---

## 6. Session/token management (operational)

The RPA needs a gov.br session token to query. How it is obtained depends on the
**mode** (`LOGIN_MODE`):

### `token` mode (recommended for VPS/production)
- The RPA **does not open a browser**. The token is captured on the **operator's
  Mac** and pushed to the RPA:
  ```bash
  # On the Mac, with the Chrome sidecar open (scripts/start-chrome.sh):
  RPA_URL=http://rpa.internal:8080 SESSION_TOKEN_APIKEY=<key> go run ./cmd/desphub-login
  ```
- The endpoint `POST /api/detran/session/token` (header `X-Api-Key`) receives it.
- Check the state anytime:
  ```bash
  curl http://rpa.internal:8080/api/detran/session
  # {"authenticated":true,"expiresAt":"...","expiresInSeconds":1799,"reportsStatus":true}
  ```
- When `expiresInSeconds` nears zero (or a query returns `503`), run the helper
  again. This can be automated with a scheduler on the Mac.

### `manual` mode (local/development)
- The RPA attaches to a real Chrome and the operator logs in the window.
- `POST /api/detran/session/reconnect` forces a new human login.

> **The backend does not need to know about tokens.** That is the RPA/operator's
> operational responsibility. The backend just calls `/queries` and handles the
> `503` when the session drops.

---

## 7. Configuration (environment variables)

| Var | Default | Description |
|-----|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `LOGIN_MODE` | *(empty)* | `token` (VPS) or `manual` (local). Empty = queries return 501 |
| `SESSION_TOKEN_APIKEY` | *(empty)* | Protects `POST /session/token` (required in `token` mode) |
| `CHROME_DEBUG_URL` | `http://localhost:9222` | Debug Chrome (manual mode) |
| `ANONYMIZE_PII` | `true` | `false` = unmasked owner CPF (production) |
| `DETRANRS_URL` | *(internal default)* | Override the DETRAN API host |
| `LOG_LEVEL` | `info` | `debug` for diagnostics |

Deploy without Docker/Go on the VPS: cross-compile on the Mac + systemd — see `deploy/`.

---

## 8. Security

- **Never expose the RPA on the public internet.** Keep it on the internal
  network; only the DespHub backend reaches it. If external access is needed, put
  it behind Nginx/Caddy with TLS + IP allowlist.
- `SESSION_TOKEN_APIKEY` protects token injection — generate with `openssl rand -hex 32`.
- The gov.br token and the owner CPF are **sensitive data**: do not log them, and
  keep `ANONYMIZE_PII` per DespHub's LGPD policy.

---

## 9. Quick examples (curl)

```bash
# Health
curl http://localhost:8080/health

# Query
curl -s -X POST http://localhost:8080/api/detran/queries \
  -H 'content-type: application/json' \
  -d '{"plate":"IDX1756","renavam":"00561040575","types":["REGISTRATION","DEBTS","RESTRICTIONS","LICENSING"]}' | python3 -m json.tool

# Session state
curl http://localhost:8080/api/detran/session
```
