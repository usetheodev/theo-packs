FROM debian:bookworm-slim AS build
WORKDIR /app
COPY . ./

FROM python:3.12-slim-bookworm
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "python -m http.server 8080"]
