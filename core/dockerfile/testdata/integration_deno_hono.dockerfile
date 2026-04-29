FROM denoland/deno:2 AS install
WORKDIR /app
COPY deno.json ./
COPY main.ts ./
RUN --mount=type=cache,target=/deno-dir,sharing=locked \
    sh -c 'deno cache main.ts'

FROM install AS build
WORKDIR /app
COPY . .

FROM denoland/deno:2
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/health || exit 1
CMD ["deno", "task", "start"]
