FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o anisearch ./cmd/main.go

# --- Runtime ---
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/anisearch .
COPY --from=builder /build/web ./web

VOLUME /config
ENV ANISEARCH_CONFIG=/config

EXPOSE 8978

ENTRYPOINT ["./anisearch"]
