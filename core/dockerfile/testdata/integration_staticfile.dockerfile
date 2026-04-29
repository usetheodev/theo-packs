FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . ./

FROM python:3.12-slim-bookworm
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["python", "-m", "http.server", "8080"]
