FROM python:3.12-bookworm AS install
WORKDIR /app
COPY requirements.txt ./
RUN --mount=type=secret,id=THEOPACKS_START_CMD \
    sh -c 'pip install --no-cache-dir -r requirements.txt'

FROM install AS build
WORKDIR /app
COPY . .

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "uvicorn main:app --host 0.0.0.0"]
