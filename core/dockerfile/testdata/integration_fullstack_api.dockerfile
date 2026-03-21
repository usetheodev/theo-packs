FROM golang:1.23-bookworm AS install
WORKDIR /app
COPY go.mod ./
RUN sh -c 'go mod download'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'go build -o /app/server .'

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/bin/bash", "-c", "/app/server"]
