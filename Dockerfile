FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/foundry ./cmd/foundry

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
COPY --chown=foundry:foundry public ./public

RUN mkdir -p /app/data/admin /app/public \
  && chown -R foundry:foundry /app

USER foundry

EXPOSE 8080

CMD ["foundry", "--config-overlay", "content/config/site.docker.yaml", "serve"]
