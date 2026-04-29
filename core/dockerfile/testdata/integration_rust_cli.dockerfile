FROM rust:1-bookworm AS install
WORKDIR /app
COPY Cargo.toml ./
RUN sh -c 'cargo fetch'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'cargo build --release --offline'
RUN sh -c 'cp target/release/rust-cli-example /app/server'

FROM debian:bookworm-slim AS packages-apt-runtime
WORKDIR /app
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'apt-get update && apt-get install -y ca-certificates'

FROM packages-apt-runtime
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/bin/bash", "-c", "/app/server"]
