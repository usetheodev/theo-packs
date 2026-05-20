# syntax=docker/dockerfile:1

# theo-packs: generated for provider "deno".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM denoland/deno:debian AS install
WORKDIR /app
COPY deno.json ./
COPY main.ts ./
RUN --mount=type=cache,target=/deno-dir,sharing=locked \
    sh -c 'deno cache main.ts'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/deno-dir,sharing=locked \
    sh -c 'deno task build'

FROM denoland/deno:debian
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/health || exit 1
CMD ["deno", "task", "start"]
