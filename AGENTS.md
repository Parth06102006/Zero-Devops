High-signal notes for automated agents working in this repo

- Work from the package you intend to operate on. Server commands belong in the server/ folder; client commands belong in client/.

- Go module: server/go.mod declares module "Zero_Devops/server" — do not move/rename server/ or change module path. Run Go commands from inside server/:
  - Example: cd server && go run ./app

- Viper requires a .env file. server/config.LoadConfig() calls viper.ReadInConfig() and will panic if .env is missing. Always create server/.env (or set equivalent env vars) before starting the server.
  - Minimal required env keys the server reads: DATABASE_HOST, DATABASE_PORT, DATABASE_USER, DATABASE_PASS, DATABASE_NAME, SERVER_ADDRESS
  - Optional but commonly referenced: OAUTH_GITHUB_CLIENT_ID, OAUTH_GITHUB_CLIENT_SECRET, OAUTH_GITHUB_REDIRECT_URL, debug, context.timeout
  - Dot-keys like "context.timeout" map to env var CONTEXT_TIMEOUT (Viper replacer converts "." -> "_").

- Database service in compose: server/docker-compose.yaml defines a Postgres service named "pgsql" and expects DATABASE_* env vars for port/user/password/name. Typical local flow:
  1. cd server
  2. Create server/.env with values (example placeholders):
     DATABASE_HOST=127.0.0.1
     DATABASE_PORT=5432
     DATABASE_USER=postgres
     DATABASE_PASS=password
     DATABASE_NAME=zero_devops
     SERVER_ADDRESS=:9090
  3. Start DB: docker compose up -d (or docker-compose up -d)  # runs the pgsql service
  4. Run server (host mode): go run ./app

- Migrations: SQL migration files live in server/migrations/*.sql and appear to use goose annotations. Do not rely on server/Makefile migrate targets without inspection — the Makefile references misc/migrations and other tooling that may be mismatched. Apply migrations with a goose-compatible tool or psql pointed at your DB. Example (psql):
  - psql "host=127.0.0.1 port=5432 user=postgres password=password dbname=zero_devops" -f server/migrations/20260518150322_create_users_table.sql

- Makefile caveats:
  - server/Makefile assumes a Unix-like shell (bash, uname) and several global tools (migrate, gotestsum, tparse, mockery, air, golangci-lint). Avoid blindly running `make` targets on Windows CMD/PowerShell or without those tools installed.

- Tests and quick verification:
  - The Makefile's tests target uses gotestsum and extra tooling; for quick runs use vanilla go test. From server/:
    - Run all tests: go test ./... -v
    - Run a single package: go test ./authorization/auth/delivery/http -v
    - Run one test by name: go test ./authorization/auth/delivery/http -run TestLogin_Success -v
  - Many unit tests use in-process mocks and do not require a running DB, but integration tests may. Inspect failing tests before starting services.

- Running the client:
  - cd client
  - Install: npm install (package-lock.json present so use npm)
  - Dev server: npm run dev
  - Build: npm run build

- Tooling versions / environment:
  - server/go.mod specifies go 1.25.0 — use a Go toolchain >=1.25 for builds.
  - The server uses golangci-lint and other developer tools (configs present). Installing them is optional unless you intend to run Makefile lint/test targets.

- Import paths and local builds:
  - The server imports local packages using the module path (e.g., "Zero_Devops/server/..."). Building from server/ with the go tool will resolve these correctly. Do not attempt to run main.go from outside the module root without adjusting GOPATH/module settings.

- Things agents commonly get wrong (be explicit):
  - Trying to start the server without creating server/.env (causes panic). Create .env first.
  - Running Makefile migrate or migrate-* targets without noticing the Makefile refers to misc/migrations while migration files are in server/migrations.
  - Running Makefile on Windows without WSL; the Makefile expects bash/Unix tooling.
  - Assuming DB is MySQL: the repo uses Postgres (server/docker-compose.yaml uses postgres:17-alpine).

- Where to look next in this repo:
  - server/README.md — architectural notes and history (example clean-arch project)
  - server/Makefile and server/.golangci.yaml — developer targets and linter rules
  - server/config/config.go and server/app/main.go — exact env keys the server expects

Keep changes minimal and prefer running Go and Node commands from their package folders.
