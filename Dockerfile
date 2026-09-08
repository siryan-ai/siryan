# --- build ---
FROM golang:1.22-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# --- runtime ---
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive \
    PORT=10000 \
    GIN_MODE=release \
    CHROME_PATH=/usr/bin/chromium \
    CHROMIUM_FLAGS="--no-sandbox --disable-gpu --disable-dev-shm-usage"

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      chromium \
      ca-certificates \
      fonts-liberation \
      fonts-dejavu-core \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

WORKDIR /app
COPY --from=build /out/api /app/api

# non-root (Chrome no-sandbox ile uyumlu)
RUN useradd -m -u 10001 siryan \
 && chown -R siryan:siryan /app
USER siryan

EXPOSE 10000
CMD ["/app/api"]