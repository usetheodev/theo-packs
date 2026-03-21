FROM node:20-bookworm AS install
WORKDIR /app
RUN sh -c 'npm install -g pnpm'
COPY package.json ./
COPY pnpm-workspace.yaml ./
COPY packages/pkg-a/package.json packages/pkg-a/
COPY packages/pkg-b/package.json packages/pkg-b/
RUN sh -c 'pnpm install'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "pnpm start"]
