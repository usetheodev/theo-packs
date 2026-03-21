FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'npm install'

FROM install AS build
WORKDIR /app
COPY . .

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "npm start"]
