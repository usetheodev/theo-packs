# syntax=docker/dockerfile:1

# theo-packs: generated for provider "unknown".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM debian:bookworm-slim AS packages-apt-runtime
WORKDIR /app
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'apt-get update && apt-get install -y libpq-dev'

FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'pip install -r requirements.txt'

FROM packages-apt-runtime
WORKDIR /app
COPY --from=install /app /app
CMD ["gunicorn", "app:app"]
