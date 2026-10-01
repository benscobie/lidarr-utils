FROM golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /out/lidarr-utils .

FROM alpine:latest@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk --no-cache add bash ca-certificates curl ffmpeg jq su-exec tzdata xmlstarlet

COPY --from=builder /out/lidarr-utils /usr/local/bin/lidarr-utils
COPY e2e /e2e

WORKDIR /e2e
ENTRYPOINT ["/bin/bash", "/e2e/runner/suite.sh"]
