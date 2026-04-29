FROM python:3.12-bookworm AS install
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.cache/pip,sharing=locked \
    sh -c 'pip install --no-cache-dir uv && uv sync --all-packages --no-dev'

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=install /app /app
COPY --from=install /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=install /usr/local/bin /usr/local/bin
ENV PATH=/app/.venv/bin:$PATH
CMD ["python", "main.py"]
