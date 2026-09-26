FROM golang:1.26.5@sha256:5822931cf78fe98a97edcf73a0c54c29fa2386b99c8136468e274ae9fab8cfba AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /out/lidarr-utils .

FROM alpine:latest@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk --no-cache add bash ca-certificates curl ffmpeg jq tzdata xmlstarlet

COPY --from=builder /out/lidarr-utils /usr/local/bin/lidarr-utils
COPY e2e /e2e

WORKDIR /e2e
ENTRYPOINT ["/bin/bash", "/e2e/runner/suite.sh"]
