FROM denoland/deno:2 AS install
WORKDIR /app
COPY deno.json ./
COPY main.ts ./
RUN sh -c 'deno cache main.ts'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'deno task build'

FROM denoland/deno:2
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "deno task start"]
