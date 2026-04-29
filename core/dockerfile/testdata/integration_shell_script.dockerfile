# syntax=docker/dockerfile:1

FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . ./

FROM debian:bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["bash", "start.sh"]
