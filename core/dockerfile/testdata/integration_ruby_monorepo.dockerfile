FROM ruby:3.3-slim-bookworm AS install
WORKDIR /app
RUN sh -c 'apt-get update && apt-get install -y --no-install-recommends build-essential && rm -rf /var/lib/apt/lists/*'
COPY Gemfile ./
RUN sh -c 'bundle config set --local path vendor/bundle'
RUN sh -c 'bundle config set --local without development:test'
RUN sh -c 'bundle install --jobs 4 --retry 3'

FROM install AS build
WORKDIR /app
COPY . .

FROM ruby:3.3-slim-bookworm
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
ENV BUNDLE_DEPLOYMENT="true"
ENV BUNDLE_PATH="vendor/bundle"
ENV BUNDLE_WITHOUT="development:test"
USER appuser
CMD ["/bin/sh", "-c", "cd apps/api && bundle exec rackup -p ${PORT:-4567} -o 0.0.0.0"]
