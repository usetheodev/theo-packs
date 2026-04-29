FROM maven:3-eclipse-temurin-21 AS install
WORKDIR /app
COPY pom.xml ./
RUN sh -c 'mvn -B -DskipTests dependency:go-offline'

FROM install AS build
WORKDIR /app
COPY . .
RUN sh -c 'mvn -B -DskipTests package'
RUN sh -c 'sh -c 'set -e; jar=$(ls target/*.jar | grep -v -- "-sources\.jar$\|-javadoc\.jar$\|original-" | head -n1); cp "$jar" /app/app.jar''

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /app/app.jar /app/app.jar
CMD ["/bin/bash", "-c", "java -jar /app/app.jar"]
