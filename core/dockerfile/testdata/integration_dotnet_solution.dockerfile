# syntax=docker/dockerfile:1

# theo-packs: generated for provider "dotnet".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM mcr.microsoft.com/dotnet/sdk:8.0 AS install
WORKDIR /app
COPY src/Api/Api.csproj src/Api/Api.csproj
COPY dotnet-solution.sln ./
RUN --mount=type=cache,target=/root/.nuget/packages,sharing=locked \
    sh -c 'dotnet restore src/Api/Api.csproj'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.nuget/packages,sharing=locked \
    sh -c 'dotnet publish src/Api/Api.csproj -c Release -o /app/publish --no-restore -p:DebugType=None -p:DebugSymbols=false'

FROM mcr.microsoft.com/dotnet/aspnet:8.0
WORKDIR /app
RUN chown app:app /app
COPY --from=build --chown=app:app /app/publish /app/publish
USER app
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/healthz || exit 1
CMD ["dotnet", "/app/Api.dll"]
