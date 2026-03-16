FROM golang:1.26-alpine AS builder

RUN apk --no-network add ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /zenith-ingestor cmd/ingestor/main.go
FROM gcr.io/distroless/static-debian12
COPY --from=builder /zenith-ingestor /zenith-ingestor

EXPOSE 8080

CMD ["/zenith-ingestor"]
