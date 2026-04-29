FROM php:8.2-cli-bookworm AS install
WORKDIR /app
RUN --mount=type=cache,target=/root/.composer/cache,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'apt-get update && apt-get install -y --no-install-recommends git unzip ca-certificates && rm -rf /var/lib/apt/lists/* && curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer'
COPY composer.json ./
RUN --mount=type=cache,target=/root/.composer/cache,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'composer install --no-dev --no-scripts --prefer-dist --optimize-autoloader --no-progress'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'php artisan config:cache || true; php artisan route:cache || true; php artisan view:cache || true'

FROM php:8.2-cli-bookworm
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/health || exit 1
CMD ["/bin/sh", "-c", "php artisan serve --host=0.0.0.0 --port=${PORT:-8000}"]
