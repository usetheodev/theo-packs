# syntax=docker/dockerfile:1

# theo-packs: generated for provider "unknown".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . .
RUN sh -c 'go build -o /app/server .'

FROM debian:bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app/server /app/server
USER appuser
CMD ["/app/server"]
