# syntax=docker/dockerfile:1

# theo-packs: generated for provider "java".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM gradle:8-jdk21 AS install
WORKDIR /app
COPY build.gradle.kts ./
COPY settings.gradle.kts ./
RUN --mount=type=cache,target=/root/.gradle,sharing=locked \
    sh -c 'gradle dependencies --no-daemon --refresh-dependencies'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.gradle,sharing=locked \
    sh -c 'gradle bootJar --no-daemon -x test'
RUN --mount=type=cache,target=/root/.gradle,sharing=locked \
    sh -c 'set -e; jar=$(ls build/libs/*.jar | grep -v -- "-plain\.jar$" | head -n1); cp "$jar" /app/app.jar'

FROM eclipse-temurin:21-jre
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app/app.jar /app/app.jar
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/actuator/health || exit 1
CMD ["java", "-jar", "/app/app.jar"]
