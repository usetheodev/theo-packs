FROM ruby:3.3-bookworm-slim AS install
WORKDIR /app
COPY Gemfile ./
RUN --mount=type=cache,target=/usr/local/bundle,sharing=locked \
    sh -c 'bundle config set --local without development:test'
RUN --mount=type=cache,target=/usr/local/bundle,sharing=locked \
    sh -c 'bundle install --jobs 4 --retry 3'

FROM install AS build
WORKDIR /app
COPY . .

FROM ruby:3.3-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
ENV BUNDLE_DEPLOYMENT="true"
ENV BUNDLE_WITHOUT="development:test"
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-4567}/health || exit 1
CMD ["/bin/sh", "-c", "bundle exec rackup -p ${PORT:-4567} -o 0.0.0.0"]
