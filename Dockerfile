FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /cobol-graph-api ./cmd/server

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /cobol-graph-api /cobol-graph-api
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

ENTRYPOINT ["/cobol-graph-api"]
