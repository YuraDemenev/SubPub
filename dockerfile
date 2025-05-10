FROM golang:1.23.9 AS build-stage
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /serve ./cmd/server/main.go

FROM debian:bookworm
WORKDIR /

COPY --from=build-stage /serve /serve
COPY config/config.yml config/config.yml
EXPOSE 8080
ENTRYPOINT ["/serve"]
