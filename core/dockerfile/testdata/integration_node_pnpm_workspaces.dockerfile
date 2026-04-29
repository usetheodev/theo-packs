FROM node:20-bookworm AS install
WORKDIR /app
RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm install -g pnpm'
COPY package.json ./
COPY pnpm-lock.yaml ./
COPY pnpm-workspace.yaml ./
COPY packages/api/package.json packages/api/
COPY packages/shared/package.json packages/shared/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'pnpm install --frozen-lockfile'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["npm", "start"]
