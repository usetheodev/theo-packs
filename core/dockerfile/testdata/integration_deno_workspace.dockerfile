FROM denoland/deno:2 AS install
WORKDIR /app
COPY deno.json ./
COPY apps/api/deno.json apps/api/deno.json

FROM install AS build
WORKDIR /app
COPY . .

FROM denoland/deno:2
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["deno", "run", "-A", "apps/api/main.ts"]
