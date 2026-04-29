FROM denoland/deno:bin-2 AS install
WORKDIR /app
COPY deno.json ./
COPY apps/api/deno.json apps/api/deno.json

FROM install AS build
WORKDIR /app
COPY . .

FROM denoland/deno:bin-2
WORKDIR /app
COPY --from=build /app /app
CMD ["/bin/bash", "-c", "deno run -A apps/api/main.ts"]
