# syntax=docker/dockerfile:1

# ---- build -------------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/seogeo ./cmd/seogeo

# ---- runtime -----------------------------------------------------------------
# Alpine + Chromium for the PDF export, with fonts covering French/English
# text, symbols and emoji.
FROM alpine:3.22
ARG VERSION=docker
LABEL org.opencontainers.image.title="SEO & GEO Report" \
      org.opencontainers.image.description="Bilingual SEO (Search Console), GA4 and GEO (AI assistants) visibility reports — MCP server for claude.ai, HTML and PDF export" \
      org.opencontainers.image.source="https://github.com/flocom/SEO-GEO-Report" \
      org.opencontainers.image.url="https://github.com/flocom/SEO-GEO-Report" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}"
RUN apk add --no-cache \
        chromium \
        font-noto \
        font-noto-emoji \
        ttf-dejavu \
        fontconfig \
        ca-certificates \
        tzdata \
    && fc-cache -f \
    && addgroup -S -g 10001 seogeo \
    && adduser -S -D -u 10001 -G seogeo -h /home/seogeo seogeo \
    && mkdir -p /data \
    && chown seogeo:seogeo /data

COPY --from=build /out/seogeo /usr/local/bin/seogeo

# Nothing has to be configured: see .env.example for the optional variables.
ENV DATA_DIR=/data \
    PORT=8080

USER seogeo
WORKDIR /home/seogeo
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["seogeo", "healthcheck"]
ENTRYPOINT ["seogeo"]
CMD ["serve"]
