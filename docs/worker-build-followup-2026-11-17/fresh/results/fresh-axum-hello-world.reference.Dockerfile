FROM docker.io/library/rust:1-alpine
WORKDIR /app
COPY . .
WORKDIR /app/.
RUN apk add --no-cache musl-dev && CARGO_BUILD_JOBS=1 cargo build --locked --release --manifest-path examples/hello-world/Cargo.toml --package example-hello-world --bin example-hello-world --target-dir /tmp/reference-target
CMD ["sh", "-c", "/tmp/reference-target/release/example-hello-world"]
