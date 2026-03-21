FROM node:20-bookworm AS install
WORKDIR /app
RUN sh -c 'corepack enable'
COPY package.json ./
COPY yarn.lock ./
COPY packages/pkg-a/package.json packages/pkg-a/
RUN sh -c 'yarn install --frozen-lockfile'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "npm start"]
