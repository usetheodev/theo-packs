# syntax=docker/dockerfile:1

FROM rust:1-bookworm AS install
WORKDIR /app
COPY Cargo.toml ./

FROM install AS build
WORKDIR /app
COPY . .
ENV RUSTFLAGS="-C strip=symbols"
RUN --mount=type=cache,target=/app/target,sharing=locked \
    --mount=type=cache,target=/root/.cargo/git,sharing=locked \
    --mount=type=cache,target=/root/.cargo/registry,sharing=locked \
    sh -c 'cargo build --release'
RUN --mount=type=cache,target=/app/target,sharing=locked \
    --mount=type=cache,target=/root/.cargo/git,sharing=locked \
    --mount=type=cache,target=/root/.cargo/registry,sharing=locked \
    sh -c 'cp target/release/rust-cli-example /app/server'

FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/app/server"]
