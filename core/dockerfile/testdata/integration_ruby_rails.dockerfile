# syntax=docker/dockerfile:1

# theo-packs: generated for provider "ruby".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM ruby:3.3-bookworm AS install
WORKDIR /app
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
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-3000}/health || exit 1
CMD ["/bin/sh", "-c", "bundle exec rails server -b 0.0.0.0 -p ${PORT:-3000} -e production"]
