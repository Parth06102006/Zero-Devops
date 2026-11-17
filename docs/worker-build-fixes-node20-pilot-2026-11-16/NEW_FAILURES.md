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

## Interpretation limits

The local HTTP probe shares the Buildah parent network namespace. A loopback
listener can pass it without being reachable from an external container.
Deployment/publication is not attempted. Preserve this distinction when
interpreting any later Rust smoke passes or worker `success` states.
