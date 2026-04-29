FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . ./

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["bash", "start.sh"]
