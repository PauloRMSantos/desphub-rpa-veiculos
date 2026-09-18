# syntax=docker/dockerfile:1

# ---- Stage 1: build ----
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/rpa ./cmd/rpa

# ---- Stage 2: runtime with Chromium ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        chromium \
        ca-certificates \
        fonts-liberation \
    && rm -rf /var/lib/apt/lists/*

# chromedp uses the system Chromium.
ENV CHROME_PATH=/usr/bin/chromium
ENV HEADLESS=true
ENV PORT=8080

COPY --from=build /out/rpa /usr/local/bin/rpa

# Non-root user.
RUN useradd -m -u 10001 rpa
USER rpa

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/rpa"]
