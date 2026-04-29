# syntax=docker/dockerfile:1

# theo-packs: generated for provider "go".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

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
