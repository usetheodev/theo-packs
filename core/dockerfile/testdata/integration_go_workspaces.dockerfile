FROM golang:1.23-bookworm AS install
WORKDIR /app
COPY go.work ./
COPY api/go.mod api/
COPY shared/go.mod shared/

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'go build -o /app/server ./api'

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/app/server"]
