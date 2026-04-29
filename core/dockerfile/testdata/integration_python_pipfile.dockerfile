# syntax=docker/dockerfile:1

FROM python:3.12-bookworm AS install
WORKDIR /app
COPY Pipfile ./
RUN --mount=type=cache,target=/root/.cache/pip,sharing=locked \
    sh -c 'pip install --no-cache-dir pipenv && pipenv requirements > requirements-pipfile.txt && pip install --no-cache-dir -r requirements-pipfile.txt'

FROM install AS build
WORKDIR /app
COPY . .

FROM python:3.12-slim-bookworm
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
COPY --from=build --chown=appuser:appuser /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=build --chown=appuser:appuser /usr/local/bin /usr/local/bin
USER appuser
CMD ["gunicorn", "-w", "4", "app:app", "--bind", "0.0.0.0:8000"]
