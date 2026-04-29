FROM rust:1-bookworm AS install
WORKDIR /app
COPY Cargo.toml ./
COPY apps/api/Cargo.toml apps/api/
COPY apps/worker/Cargo.toml apps/worker/
COPY packages/shared/Cargo.toml packages/shared/
RUN --mount=type=secret,id=THEOPACKS_APP_NAME \
    sh -c 'cargo fetch'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=secret,id=THEOPACKS_APP_NAME \
    sh -c 'cargo build --release --offline -p api'
RUN --mount=type=secret,id=THEOPACKS_APP_NAME \
    sh -c 'cp target/release/api /app/server'

FROM debian:bookworm-slim AS packages-apt-runtime
WORKDIR /app
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'apt-get update && apt-get install -y ca-certificates'

FROM packages-apt-runtime
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/bin/bash", "-c", "/app/server"]
