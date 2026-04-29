FROM denoland/deno:2 AS install
WORKDIR /app
COPY deno.json ./
COPY main.ts ./
RUN --mount=type=cache,target=/deno-dir,sharing=locked \
    sh -c 'deno cache main.ts'

FROM install AS build
WORKDIR /app
COPY . .

FROM denoland/deno:2
WORKDIR /app
COPY --from=build /app /app
CMD ["deno", "task", "start"]
