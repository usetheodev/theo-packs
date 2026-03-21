FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . .
COPY . .

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "bash start.sh"]
