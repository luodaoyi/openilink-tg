# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25.3

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
	-trimpath \
	-ldflags="-s -w -X openilink-tg/internal/version.Version=${VERSION} -X openilink-tg/internal/version.Commit=${COMMIT} -X openilink-tg/internal/version.BuildDate=${BUILD_DATE}" \
	-o /out/openilink-tg ./cmd/openilink-tg

FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app \
	&& apk add --no-cache ca-certificates \
	&& mkdir -p /data /app \
	&& chown -R app:app /data /app

WORKDIR /app

COPY --from=builder /out/openilink-tg /app/openilink-tg

RUN chown app:app /app/openilink-tg

USER app

ENV HTTP_ADDR=:8080
ENV STATE_FILE=/data/state.json
ENV MONITOR_ENABLED=true

EXPOSE 8080

ENTRYPOINT ["/app/openilink-tg"]
