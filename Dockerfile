FROM golang:1.25-alpine AS builder

WORKDIR /src

ARG FOUNDRY_BUILD_VERSION=""
ARG FOUNDRY_BUILD_COMMIT=""
ARG FOUNDRY_BUILD_DATE=""

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go run ./cmd/plugin-sync
RUN set -eu; \
  LDFLAGS="-s -w"; \
  if [ -n "$FOUNDRY_BUILD_VERSION" ]; then \
    LDFLAGS="$LDFLAGS -X github.com/sphireinc/foundry/internal/commands/version.Version=$FOUNDRY_BUILD_VERSION"; \
  fi; \
  if [ -n "$FOUNDRY_BUILD_COMMIT" ]; then \
    LDFLAGS="$LDFLAGS -X github.com/sphireinc/foundry/internal/commands/version.Commit=$FOUNDRY_BUILD_COMMIT"; \
  fi; \
  if [ -n "$FOUNDRY_BUILD_DATE" ]; then \
    LDFLAGS="$LDFLAGS -X github.com/sphireinc/foundry/internal/commands/version.Date=$FOUNDRY_BUILD_DATE"; \
  fi; \
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o /out/foundry ./cmd/foundry

FROM alpine:3.20

RUN addgroup -S foundry && adduser -S -G foundry foundry \
  && apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/foundry /usr/local/bin/foundry
COPY --chown=foundry:foundry content ./content
COPY --chown=foundry:foundry data ./data
COPY --chown=foundry:foundry themes ./themes
COPY --chown=foundry:foundry plugins ./plugins
COPY --chown=foundry:foundry sdk ./sdk

RUN mkdir -p /app/content /app/data/admin /app/themes /app/plugins /app/public /tmp \
  && chown -R foundry:foundry /app

USER foundry

EXPOSE 8080

CMD ["foundry", "--config-overlay", "content/config/site.docker.yaml", "serve"]
