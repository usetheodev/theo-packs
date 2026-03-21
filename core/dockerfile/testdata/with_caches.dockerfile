FROM debian:bookworm-slim AS packages-apt-runtime
WORKDIR /app
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'apt-get update && apt-get install -y libpq-dev'

FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'pip install -r requirements.txt'

FROM packages-apt-runtime
WORKDIR /app
COPY --from=install /app /app
CMD ["/bin/bash", "-c", "gunicorn app:app"]
