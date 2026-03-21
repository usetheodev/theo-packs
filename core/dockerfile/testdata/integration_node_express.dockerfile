FROM node:20-bookworm AS install
WORKDIR /app
COPY package.json ./
RUN sh -c 'npm install'

FROM install AS build
WORKDIR /app
COPY . .

FROM node:20-bookworm-slim
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "npm start"]
