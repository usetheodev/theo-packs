FROM php:8.2-cli-bookworm AS install
WORKDIR /app
RUN sh -c 'apt-get update && apt-get install -y --no-install-recommends git unzip ca-certificates && rm -rf /var/lib/apt/lists/* && curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer'
COPY composer.json ./
RUN sh -c 'composer install --no-dev --no-scripts --prefer-dist --optimize-autoloader --no-progress'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'php artisan config:cache || true; php artisan route:cache || true; php artisan view:cache || true'

FROM php:8.2-cli-bookworm
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "php artisan serve --host=0.0.0.0 --port=${PORT:-8000}"]
