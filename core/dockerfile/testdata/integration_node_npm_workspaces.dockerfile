FROM node:20-bookworm AS install
WORKDIR /app
COPY package.json ./
COPY package-lock.json ./
COPY packages/api/package.json packages/api/
COPY packages/shared/package.json packages/shared/
RUN sh -c 'npm ci'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "npm start"]
