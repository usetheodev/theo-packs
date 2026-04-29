# syntax=docker/dockerfile:1

FROM denoland/deno:debian AS install
WORKDIR /app
COPY deno.json ./
COPY main.ts ./
RUN --mount=type=cache,target=/deno-dir,sharing=locked \
    sh -c 'deno cache main.ts'

FROM install AS build
WORKDIR /app
COPY . .

FROM denoland/deno:debian
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/health || exit 1
CMD ["deno", "task", "start"]
