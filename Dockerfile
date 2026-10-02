ARG BUILDPLATFORM

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH
ARG FOUNDRY_BUILD_VERSION=""
ARG FOUNDRY_BUILD_TAG=""
ARG FOUNDRY_BUILD_COMMIT=""
ARG FOUNDRY_BUILD_DATE=""

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go run ./cmd/plugin-sync
RUN set -eu; \
  LDFLAGS="-s -w -X github.com/sphireinc/foundry/internal/commands/version.ContainerImage=$FOUNDRY_BUILD_VERSION -X github.com/sphireinc/foundry/internal/commands/version.BuildTag=$FOUNDRY_BUILD_TAG"; \
  if [ -n "$FOUNDRY_BUILD_VERSION" ]; then \
    LDFLAGS="$LDFLAGS -X github.com/sphireinc/foundry/internal/commands/version.Version=$FOUNDRY_BUILD_VERSION"; \
  fi; \
  if [ -n "$FOUNDRY_BUILD_COMMIT" ]; then \
    LDFLAGS="$LDFLAGS -X github.com/sphireinc/foundry/internal/commands/version.Commit=$FOUNDRY_BUILD_COMMIT"; \
  fi; \
  if [ -n "$FOUNDRY_BUILD_DATE" ]; then \
    LDFLAGS="$LDFLAGS -X github.com/sphireinc/foundry/internal/commands/version.Date=$FOUNDRY_BUILD_DATE"; \
  fi; \
  CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH:-amd64}" go build -trimpath -ldflags="$LDFLAGS" -o /out/foundry ./cmd/foundry

# The static image is intended for a checked-out Foundry site mounted at
# /repo. It deliberately keeps the build command overridable so callers can
# use `foundry build`, `foundry validate`, or another bounded CLI command.
FROM alpine:3.20 AS static
ENV FOUNDRY_CONTAINER=true

ARG FOUNDRY_BUILD_VERSION=""
ARG FOUNDRY_BUILD_COMMIT=""
ARG FOUNDRY_BUILD_DATE=""

LABEL org.opencontainers.image.title="Foundry static builder" \
      org.opencontainers.image.description="Foundry static-site build image" \
      org.opencontainers.image.source="https://github.com/sphireinc/Foundry" \
      org.opencontainers.image.version="$FOUNDRY_BUILD_VERSION" \
      org.opencontainers.image.revision="$FOUNDRY_BUILD_COMMIT" \
      org.opencontainers.image.created="$FOUNDRY_BUILD_DATE"

RUN addgroup -S foundry && adduser -S -G foundry foundry \
  && apk add --no-cache ca-certificates tzdata

WORKDIR /repo

COPY --from=builder /out/foundry /usr/local/bin/foundry

# The image user is non-root. When /repo is a host bind mount, callers can
# pass --user "$(id -u):$(id -g)" so generated public/ files are owned by the
# invoking user.
USER foundry

CMD ["foundry", "build"]

FROM alpine:3.20 AS runtime
ENV FOUNDRY_CONTAINER=true

ARG FOUNDRY_BUILD_VERSION=""
ARG FOUNDRY_BUILD_COMMIT=""
ARG FOUNDRY_BUILD_DATE=""

LABEL org.opencontainers.image.title="Foundry" \
      org.opencontainers.image.description="Markdown-first CMS runtime" \
      org.opencontainers.image.source="https://github.com/sphireinc/Foundry" \
      org.opencontainers.image.version="$FOUNDRY_BUILD_VERSION" \
      org.opencontainers.image.revision="$FOUNDRY_BUILD_COMMIT" \
      org.opencontainers.image.created="$FOUNDRY_BUILD_DATE"

RUN addgroup -S foundry && adduser -S -G foundry foundry \
  && apk add --no-cache ca-certificates tzdata wget

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
