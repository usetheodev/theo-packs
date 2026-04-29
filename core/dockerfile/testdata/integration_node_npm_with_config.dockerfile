FROM node:20-bookworm AS install
WORKDIR /app
COPY package.json ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm install'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["node", "server.js"]
