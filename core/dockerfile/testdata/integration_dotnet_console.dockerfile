FROM mcr.microsoft.com/dotnet/sdk:8.0 AS install
WORKDIR /app
COPY dotnet-console.csproj dotnet-console.csproj
RUN sh -c 'dotnet restore dotnet-console.csproj'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'dotnet publish dotnet-console.csproj -c Release -o /app/publish --no-restore'

FROM mcr.microsoft.com/dotnet/runtime:8.0
WORKDIR /app
COPY --from=build /app/publish /app/publish
CMD ["dotnet", "/app/dotnet-console.dll"]
