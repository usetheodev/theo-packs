# syntax=docker/dockerfile:1

# theo-packs: generated for provider "rust".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

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
    sh -c 'cp target/release/rust-axum-example /app/server'

FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app
COPY --from=build /app/server /app/server
CMD ["/app/server"]
