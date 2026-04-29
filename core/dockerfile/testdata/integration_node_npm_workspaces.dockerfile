# syntax=docker/dockerfile:1

FROM node:20-bookworm AS install
WORKDIR /app
COPY package.json ./
COPY packages/api/package.json packages/api/
COPY packages/shared/package.json packages/shared/
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm install'

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
