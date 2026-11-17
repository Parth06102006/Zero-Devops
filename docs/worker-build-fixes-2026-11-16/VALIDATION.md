# Validation scope

The final tested product source is commit
`20544b6c95b9cb1536250f2d8a7fef24560ceb04`. `execution-source.json` records the
worker binary hash, source-file hashes, and template hashes. The running binary
hash is independently recorded in `running-worker-sha256.txt`.

The worker binary was compiled locally with the Go toolchain recorded in
`build-toolchain.txt`. It and the committed templates were mounted read-only
into the retained baseline test runtime image, whose immutable image ID is in
`environment.json`. The worker's Dockerfile was not rebuilt in this run. The
runtime's actual Buildah version and configured resource limits are recorded
in `buildah-version.txt` and `resource-configuration.json`.

The complete worker-server suite passes in
`worker-suite-with-native-tests.log`. Host tests skip live Buildah tests when
Buildah is unavailable. The separate `native-build-integration.log` demonstrates
real image builds and CLI startup for the final implementation:

- A Node app requiring `>=22.23.4`, with actual-image engine checks in both
  frozen dependency-install stages and successful execution of its built file.
- A Go app whose executable is in `./cmd/server`, with the app directory
  distinct from its repository context.
- A standalone Rust package with multiple binaries, selecting and executing
  the configured `admin` binary rather than another executable.
- A Rust workspace member inheriting package fields and using a sibling path
  dependency, selecting and executing its specific binary.

These fixtures are distinct from the 57-case application campaign and do not
inflate its denominator. Their CLI execution is not HTTP or deployment success.

All eight affected image references passed pulls in an initially empty,
separate Buildah store (`cold-pulls.json`). Pulled Node/Go/Rust base-image
metadata is retained in the corresponding `cold-*-image.json` files. No images
from that store were copied into the worker's initially empty store.

The final campaign reuses the original 57 cases' recorded reference eligibility
and restores their immutable SHAs through the worker's exact-commit clone path.
Reference builds were not repeated. The final image, local HTTP smoke, worker
database state, and fake upload are recorded separately. No real publication
or deployment is attempted.

The interim Node-20 pilot is retained in the sibling directory
`../worker-build-fixes-node20-pilot-2026-11-16/`. It exposed the Astro engine
requirement and is excluded from the final-source comparison. An initial TLS
handshake timeout while pulling Node 24 for the final fixture retried
successfully; the original log is retained.

The user later approved up to four concurrent cases and a 6 CPU / 8 GiB builder budget. Due to the VM’s reported 5.6 GiB total memory, the final continuation uses two reference-sized builders, with separate stores/networks/vhosts/mock endpoints and a shared run-owned test DB/broker. The initial 17 completed records are preserved. Actual lane limits, empty lane-2 initial storage, and its matching binary hash are in lanes/lane2.json; scheduling and capacity are in parallel-state.json. Product source hashes remain unchanged.
