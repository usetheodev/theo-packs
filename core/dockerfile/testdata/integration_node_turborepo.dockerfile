FROM node:20-bookworm AS install
WORKDIR /app
COPY package.json ./
COPY package-lock.json ./
COPY turbo.json ./
COPY apps/api/package.json apps/api/
COPY apps/web/package.json apps/web/
COPY packages/ui/package.json packages/ui/
COPY packages/utils/package.json packages/utils/
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm ci'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["npm", "start"]
