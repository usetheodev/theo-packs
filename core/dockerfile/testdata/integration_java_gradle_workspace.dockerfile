FROM gradle:8-jdk21 AS install
WORKDIR /app
COPY build.gradle.kts ./
COPY settings.gradle.kts ./

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=secret,id=THEOPACKS_APP_NAME \
    sh -c 'gradle :apps:api:bootJar --no-daemon -x test'
RUN --mount=type=secret,id=THEOPACKS_APP_NAME \
    sh -c 'sh -c 'set -e; jar=$(ls apps/api/build/libs/*.jar | grep -v -- "-plain\.jar$" | head -n1); cp "$jar" /app/app.jar''

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /app/app.jar /app/app.jar
CMD ["/bin/bash", "-c", "java -jar /app/app.jar"]
