FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'npm install'

FROM install AS build
WORKDIR /app
COPY . .

FROM debian:bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["npm", "start"]
