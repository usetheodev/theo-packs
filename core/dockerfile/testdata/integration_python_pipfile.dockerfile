FROM python:3.12-bookworm AS install
WORKDIR /app
COPY Pipfile ./
RUN --mount=type=secret,id=THEOPACKS_START_CMD \
    sh -c 'pip install --no-cache-dir pipenv && pipenv install --deploy --system'

FROM install AS build
WORKDIR /app
COPY . .

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "python main.py"]
