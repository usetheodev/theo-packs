# syntax=docker/dockerfile:1

# theo-packs: generated for provider "dotnet".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM mcr.microsoft.com/dotnet/sdk:8.0 AS install
WORKDIR /app
COPY dotnet-console.csproj dotnet-console.csproj
RUN --mount=type=cache,target=/root/.nuget/packages,sharing=locked \
    sh -c 'dotnet restore dotnet-console.csproj'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.nuget/packages,sharing=locked \
    sh -c 'dotnet publish dotnet-console.csproj -c Release -o /app/publish --no-restore -p:DebugType=None -p:DebugSymbols=false'

FROM mcr.microsoft.com/dotnet/runtime:8.0-alpine
WORKDIR /app
RUN chown app:app /app
COPY --from=build --chown=app:app /app/publish /app/publish
USER app
CMD ["dotnet", "/app/dotnet-console.dll"]
