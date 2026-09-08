FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM chromedp/headless-shell:stable

ENV PORT=10000 \
    GIN_MODE=release \
    CHROME_PATH=/headless-shell/headless-shell

USER root
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/api /app/api
RUN chmod +x /app/api

# ÖNEMLİ: base image ENTRYPOINT'ini iptal et
ENTRYPOINT []
CMD ["/app/api"]

EXPOSE 10000