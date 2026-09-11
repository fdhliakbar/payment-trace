FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
WORKDIR /src
COPY payment-dotnet.csproj .
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app/publish /p:UseAppHost=false

FROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine
WORKDIR /app
COPY --from=build /app/publish .
ENV ASPNETCORE_URLS=http://0.0.0.0:8083
ENV PORT=8083
EXPOSE 8083
ENTRYPOINT ["dotnet", "payment-dotnet.dll"]
