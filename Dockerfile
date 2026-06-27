FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY config ./config
COPY errors ./errors
COPY http ./http
COPY logger ./logger
COPY models ./models
COPY repositories ./repositories
COPY services ./services
COPY utils ./utils

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o test_app ./cmd/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /app/test_app /app/test_app

USER app

EXPOSE 8080

ENTRYPOINT ["/app/test_app"]
