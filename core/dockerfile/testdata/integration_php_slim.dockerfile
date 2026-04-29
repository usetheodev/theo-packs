FROM php:8.1-cli-bookworm AS install
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

FROM php:8.1-cli-bookworm
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/sh", "-c", "php -S 0.0.0.0:${PORT:-8000} -t public"]
