# Failures exposed after the original blockers

These observations supplement first-attempt result JSON and logs; they do not
replace failures with controlled successes or change the campaign denominator.

## Vite Svelte

`vite-svelte` and `vite-svelte-ts` previously stopped at `npm ci` without a lockfile. With the
explicit unlocked fallback, dependency installation succeeds and `vite build`
produces `dist/index.html`. Detection selected the SvelteKit runtime recipe,
whose final stage tries to copy `/app/build`. That copy fails because the app
produced `dist`, so no final image exists. This is a newly exposed framework /
output-recipe mismatch, outside the three targeted fixes.

Evidence: [worker log](results/vite-svelte.worker.log),
[case record](results/vite-svelte.json), and the [TypeScript worker log](results/vite-svelte-ts.worker.log). The build log distinguishes successful
compilation from failure to create the final image.

## Actix static files: image success, missing runtime assets

`actix-static-files` now builds its selected binary with the workspace root and
lockfile present. The final image exists and starts the server, but the recorded
HTTP probe returns 404. Runtime logs report missing `static/images/` and
`./static/root/` directories. The runtime recipe copies the selected binary into
`/app` and does not carry these application asset directories. This is a
separate runtime packaging/work-directory limitation, not a workspace build
failure, and was not repaired by changing the sample app or broadening the
three targeted fixes.

Evidence: [case record](results/actix-static-files.json),
[runtime log](results/actix-static-files.worker-runtime.log).

## Astro with Svelte: compilation passes, wrong runtime recipe

`astro-framework-svelte` installs under the selected Node 24 image and
`astro build` successfully emits `dist/index.html`. The detector selects the
SvelteKit recipe, which then attempts to copy `/app/build` and fails. The actual
engine checks pass in both install stages. This exposes the same Svelte config
precedence/output-recipe issue as the Vite Svelte starters, rather than a
remaining lockfile or Node-version failure.

Evidence: [worker log](results/astro-framework-svelte.worker.log),
[case record](results/astro-framework-svelte.json).

## Next API routes: optional public directory assumed mandatory

`next-api-routes-rest` installs and compiles successfully, including its API
routes. The runtime recipe then unconditionally copies `/app/public`, which
does not exist in this example. Final image creation fails at that copy. The
successful compilation is not counted as image creation or runtime success.

Evidence: [worker log](results/next-api-routes-rest.worker.log),
[case record](results/next-api-routes-rest.json).

## Git fetch / checkout deadlines

`next-basic-css` and `next-with-typescript` hit the worker's existing 60-second
Git clone budget during checkout and fetch respectively. Those trials do not
test their application compilation or runtime. They remain failures in the
57-case primary denominator; they are not treated as intrinsically broken
repositories or silently replaced by repeat outcomes.

Evidence: [CSS worker log](results/next-basic-css.worker.log),
[TypeScript worker log](results/next-with-typescript.worker.log).

## Probe limitations

The local HTTP probe shares the Buildah parent network namespace. A loopback
listener can pass it without being reachable from an external container.
Deployment/publication is not attempted. Preserve this distinction when
interpreting any later Rust smoke passes or worker `success` states.
