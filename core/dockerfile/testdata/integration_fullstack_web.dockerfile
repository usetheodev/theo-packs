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
    sh -c 'npm run build'

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["npm", "start"]
