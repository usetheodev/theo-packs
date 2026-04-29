FROM mcr.microsoft.com/dotnet/sdk:8.0 AS install
WORKDIR /app
COPY dotnet-aspnet.csproj dotnet-aspnet.csproj
RUN sh -c 'dotnet restore dotnet-aspnet.csproj'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'dotnet publish dotnet-aspnet.csproj -c Release -o /app/publish --no-restore'

FROM mcr.microsoft.com/dotnet/aspnet:8.0
WORKDIR /app
COPY --from=build /app/publish /app/publish
CMD ["/bin/bash", "-c", "dotnet /app/dotnet-aspnet.dll"]
