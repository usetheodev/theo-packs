FROM python:3.12-bookworm AS install
WORKDIR /app
COPY requirements.txt ./
RUN --mount=type=cache,target=/root/.cache/pip,sharing=locked \
    sh -c 'pip install --no-cache-dir -r requirements.txt'

FROM install AS build
WORKDIR /app
COPY . .

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=build /app /app
COPY --from=build /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=build /usr/local/bin /usr/local/bin
CMD ["gunicorn", "myproject.wsgi:application", "--bind", "0.0.0.0:8000"]
