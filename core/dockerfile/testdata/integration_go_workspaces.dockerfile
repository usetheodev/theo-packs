# syntax=docker/dockerfile:1

FROM golang:1.23-bookworm AS install
WORKDIR /app
COPY go.work ./
COPY api/go.mod api/
COPY shared/go.mod shared/

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    sh -c 'go build -ldflags="-s -w" -trimpath -o /app/server ./api'

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/app/server"]
