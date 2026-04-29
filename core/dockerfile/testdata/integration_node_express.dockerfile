# syntax=docker/dockerfile:1

# theo-packs: generated for provider "node".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM node:20-bookworm AS install
WORKDIR /app
COPY package.json ./
COPY package-lock.json ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm ci'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm prune --omit=dev'

FROM node:20-bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["npm", "start"]
