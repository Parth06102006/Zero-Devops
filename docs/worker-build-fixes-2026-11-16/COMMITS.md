# Local implementation commits

Base: `f01968cf3c453781fe74936ee51c2e000b327107` (`main`, safely fetched and
fast-forward checked before creating `fix-builds`; already up to date).

| Commit | Change |
|---|---|
| `ae784ded9d4eb772e364c925837c9209f751574c` | Explicit unlocked npm policy, frozen/shrinkwrap handling, qualified image references, Go requirement/target selection, Cargo context/package/binary selection, containment checks, focused regressions, initial replay tooling. |
| `7d853dc` | Passing live native fixtures, campaign/report tooling, and accurate Buildah/publication documentation. |
| `20544b6c95b9cb1536250f2d8a7fef24560ceb04` | Node engine/pin selection and actual-image checks, Node fixture and regressions, default Node 24 templates, and final isolated harness preparation. |

The final 57-case replay tests the product source at `20544b6`. The Node-20
interim pilot is kept separately with its original metadata. No commits were
pushed, no pull request was published, and no deployment was performed.
