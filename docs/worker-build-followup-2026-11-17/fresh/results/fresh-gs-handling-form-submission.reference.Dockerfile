FROM docker.io/library/maven:3-eclipse-temurin-21
WORKDIR /app
COPY . .
WORKDIR /app/.
RUN mvn -q -DskipTests package
CMD ["sh", "-c", "java -jar target/handling-form-submission-complete-0.0.1-SNAPSHOT.jar"]
