# syntax=docker/dockerfile:1

# theo-packs: generated for provider "staticfile".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . ./

FROM python:3.12-slim-bookworm
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["python", "-m", "http.server", "8080"]
