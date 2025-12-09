# syntax=docker/dockerfile:1.6

FROM golang:1.23 AS builder
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api ./cmd/api

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/api ./api
COPY ./.env.example ./env.sample
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
