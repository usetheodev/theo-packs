FROM node:20-bookworm AS install
WORKDIR /app
COPY . .
RUN sh -c 'npm install'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim AS packages-apt-runtime
WORKDIR /app
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'apt-get update && apt-get install -y curl'

FROM packages-apt-runtime
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "node server.js"]
