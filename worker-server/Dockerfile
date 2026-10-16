# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -o /out/worker-engine ./cmd/worker-server

# Buildah requires a Linux runtime. The Compose service runs this image
# privileged for local Docker Desktop testing.
FROM quay.io/buildah/stable:v1.39
RUN dnf --assumeyes install git ca-certificates && dnf clean all

WORKDIR /app
COPY --from=build /out/worker-engine ./worker-engine
COPY templates ./templates

ENTRYPOINT ["/app/worker-engine"]
