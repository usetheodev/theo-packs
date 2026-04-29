FROM rust:1-bookworm AS install
WORKDIR /app
COPY Cargo.toml ./
COPY apps/api/Cargo.toml apps/api/
COPY apps/worker/Cargo.toml apps/worker/
COPY packages/shared/Cargo.toml packages/shared/

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/app/target,sharing=locked \
    --mount=type=cache,target=/root/.cargo/git,sharing=locked \
    --mount=type=cache,target=/root/.cargo/registry,sharing=locked \
    sh -c 'cargo build --release -p api'
RUN --mount=type=cache,target=/app/target,sharing=locked \
    --mount=type=cache,target=/root/.cargo/git,sharing=locked \
    --mount=type=cache,target=/root/.cargo/registry,sharing=locked \
    sh -c 'cp target/release/api /app/server'

FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/app/server"]
