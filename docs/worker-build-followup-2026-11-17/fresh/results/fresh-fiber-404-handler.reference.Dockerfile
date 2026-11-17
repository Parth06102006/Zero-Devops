FROM docker.io/library/golang:1.27.0-alpine
WORKDIR /app
COPY . .
WORKDIR /app/.
RUN GOTOOLCHAIN=local go build -o /tmp/reference-server .
CMD ["sh", "-c", "/tmp/reference-server"]
