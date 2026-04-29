FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . .
RUN sh -c 'go build -o /app/server .'

FROM debian:bookworm-slim
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app/server /app/server
USER appuser
CMD ["/app/server"]
