# syntax=docker/dockerfile:1

# theo-packs: generated for provider "python".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM python:3.12-bookworm AS install
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.cache/pip,sharing=locked \
    sh -c 'pip install --no-cache-dir uv && uv sync --all-packages --no-dev'

FROM python:3.12-slim-bookworm
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=install --chown=appuser:appuser /app /app
COPY --from=install --chown=appuser:appuser /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=install --chown=appuser:appuser /usr/local/bin /usr/local/bin
ENV PATH=/app/.venv/bin:$PATH
USER appuser
CMD ["python", "main.py"]
