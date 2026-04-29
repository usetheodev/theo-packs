FROM python:3.12-bookworm AS install
WORKDIR /app
COPY pyproject.toml ./
COPY poetry.lock ./
RUN sh -c 'pip install --no-cache-dir poetry && poetry config virtualenvs.create false && poetry install --no-root --no-interaction --no-ansi'

FROM install AS build
WORKDIR /app
COPY . .

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=build /app /app
COPY --from=build /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=build /usr/local/bin /usr/local/bin
CMD ["gunicorn", "-w", "4", "app:app", "--bind", "0.0.0.0:8000"]
