FROM maven:3-eclipse-temurin-21 AS install
WORKDIR /app
COPY pom.xml ./
RUN --mount=type=cache,target=/root/.m2,sharing=locked \
    sh -c 'mvn -B -DskipTests dependency:go-offline'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.m2,sharing=locked \
    sh -c 'mvn -B -DskipTests package'
RUN --mount=type=cache,target=/root/.m2,sharing=locked \
    sh -c 'set -e; jar=$(ls target/*.jar | grep -v -- "-sources\.jar$\|-javadoc\.jar$\|original-" | head -n1); cp "$jar" /app/app.jar'

FROM eclipse-temurin:21-jre
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app/app.jar /app/app.jar
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/actuator/health || exit 1
CMD ["java", "-jar", "/app/app.jar"]
