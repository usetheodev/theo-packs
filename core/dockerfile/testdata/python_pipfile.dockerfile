# syntax=docker/dockerfile:1

FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'pip install pipenv && pipenv install --deploy --system'

FROM debian:bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=install --chown=appuser:appuser /app /app
USER appuser
CMD ["python", "main.py"]
