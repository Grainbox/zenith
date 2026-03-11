FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /zenith-ingestor cmd/ingestor/main.go
FROM scratch
COPY --from=builder /zenith-ingestor /zenith-ingestor

EXPOSE 50051

CMD ["/zenith-ingestor"]
