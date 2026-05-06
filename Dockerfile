FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o viofo-backup ./cmd/viofo-backup

FROM alpine:3.21
RUN adduser -D -u 1000 app
WORKDIR /app
COPY --from=builder /build/viofo-backup .
USER app
ENTRYPOINT ["/app/viofo-backup"]
CMD ["--config", "/app/config.yaml"]
