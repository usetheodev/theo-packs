FROM python:3.12-bookworm AS install
WORKDIR /app
COPY . .
RUN --mount=type=secret,id=THEOPACKS_START_CMD \
    sh -c 'pip install --no-cache-dir uv && uv sync --no-dev'

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=install /app /app
CMD ["/bin/bash", "-c", "python main.py"]
