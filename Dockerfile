# syntax=docker/dockerfile:1

FROM golang:1.22 AS builder
WORKDIR /src

# Pre-fetch modules for better caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bot ./cmd/bot

FROM gcr.io/distroless/static-debian12 AS runner
WORKDIR /app

COPY --from=builder /bot ./bot

USER nonroot:nonroot
ENTRYPOINT ["/app/bot"]
