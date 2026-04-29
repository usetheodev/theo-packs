FROM mcr.microsoft.com/dotnet/sdk:8.0 AS install
WORKDIR /app
COPY src/Api/Api.csproj src/Api/Api.csproj
COPY dotnet-solution.sln ./
RUN sh -c 'dotnet restore src/Api/Api.csproj'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'dotnet publish src/Api/Api.csproj -c Release -o /app/publish --no-restore'

FROM mcr.microsoft.com/dotnet/aspnet:8.0
WORKDIR /app
COPY --from=build /app/publish /app/publish
CMD ["dotnet", "/app/Api.dll"]
