FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . .
COPY . .

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["python", "-m", "http.server", "80"]
