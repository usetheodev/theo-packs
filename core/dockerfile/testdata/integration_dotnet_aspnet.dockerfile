FROM mcr.microsoft.com/dotnet/sdk:8.0 AS install
WORKDIR /app
COPY dotnet-aspnet.csproj dotnet-aspnet.csproj
RUN --mount=type=cache,target=/root/.nuget/packages,sharing=locked \
    sh -c 'dotnet restore dotnet-aspnet.csproj'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.nuget/packages,sharing=locked \
    sh -c 'dotnet publish dotnet-aspnet.csproj -c Release -o /app/publish --no-restore'

FROM mcr.microsoft.com/dotnet/aspnet:8.0
WORKDIR /app
COPY --from=build /app/publish /app/publish
CMD ["dotnet", "/app/dotnet-aspnet.dll"]
