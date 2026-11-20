# Campaign versions

- `50-case/` contains the pinned baseline campaign protocol and manifest. Its findings informed worker and template fixes.
- `broad-compatibility/` contains the later diagnostic runners, queue snapshot, and source report.
- Shared runners and environment helpers remain in `scripts/worker-build-campaign/` so versioned runners can import them.

Raw result folders remain on disk under `docs/worker-build-*` and are ignored by Git. The consolidated, human-readable report is `docs/FINAL_REPORT.md`.
