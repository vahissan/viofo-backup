FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -o viofo-backup ./cmd/viofo-backup

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /build/viofo-backup .
ENTRYPOINT ["/app/viofo-backup"]
CMD ["--config", "/app/config.yaml"]
