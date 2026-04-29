# syntax=docker/dockerfile:1

FROM denoland/deno:debian AS install
WORKDIR /app
COPY deno.json ./
COPY apps/api/deno.json apps/api/deno.json

FROM install AS build
WORKDIR /app
COPY . .

FROM denoland/deno:debian
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["deno", "run", "-A", "apps/api/main.ts"]
