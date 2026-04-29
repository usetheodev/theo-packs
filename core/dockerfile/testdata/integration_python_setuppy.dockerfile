FROM python:3.12-bookworm AS install
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.cache/pip,sharing=locked \
    sh -c 'pip install --no-cache-dir .'

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=install /app /app
COPY --from=install /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=install /usr/local/bin /usr/local/bin
CMD ["myapp"]
