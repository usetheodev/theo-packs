FROM node:20-bookworm AS install
WORKDIR /app
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    --mount=type=cache,target=/usr/local/share/.cache/yarn,sharing=locked \
    sh -c 'corepack enable'
COPY package.json ./
COPY yarn.lock ./
COPY packages/api/package.json packages/api/
COPY packages/shared/package.json packages/shared/
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    --mount=type=cache,target=/usr/local/share/.cache/yarn,sharing=locked \
    sh -c 'yarn install --frozen-lockfile'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["npm", "start"]
