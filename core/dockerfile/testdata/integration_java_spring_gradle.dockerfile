FROM gradle:8-jdk21 AS install
WORKDIR /app
COPY build.gradle.kts ./
COPY settings.gradle.kts ./

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.gradle,sharing=locked \
    sh -c 'gradle bootJar --no-daemon -x test'
RUN --mount=type=cache,target=/root/.gradle,sharing=locked \
    sh -c 'set -e; jar=$(ls build/libs/*.jar | grep -v -- "-plain\.jar$" | head -n1); cp "$jar" /app/app.jar'

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /app/app.jar /app/app.jar
CMD ["java", "-jar", "/app/app.jar"]
