FROM ruby:3.3-bookworm-slim AS install
WORKDIR /app
COPY Gemfile ./
RUN sh -c 'bundle config set --local without development:test'
RUN sh -c 'bundle install --jobs 4 --retry 3'

FROM install AS build
WORKDIR /app
COPY . .

FROM ruby:3.3-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
ENV BUNDLE_DEPLOYMENT="true"
ENV BUNDLE_WITHOUT="development:test"
CMD ["/bin/sh", "-c", "bundle exec rails server -b 0.0.0.0 -p ${PORT:-3000} -e production"]
