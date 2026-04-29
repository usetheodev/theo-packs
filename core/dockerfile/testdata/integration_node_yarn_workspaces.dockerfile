FROM node:20-bookworm AS install
WORKDIR /app
RUN sh -c 'corepack enable'
COPY package.json ./
COPY yarn.lock ./
COPY packages/api/package.json packages/api/
COPY packages/shared/package.json packages/shared/
RUN sh -c 'yarn install --frozen-lockfile'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["npm", "start"]
