FROM golang:1.27.1@sha256:162be5298a40ed317005c8339c6de4d10d3eef336d66dc8e9259b03ab9d3a6d2 AS builder

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
