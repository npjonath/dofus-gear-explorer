# Dofus Gear Explorer

pnpm monorepo: a Go API (`apps/api`), a shared React interface (`packages/app`) and two thin shells that mount it, web (`apps/web`) and Electron (`apps/desktop`).

## Commands

- `pnpm dev`: API on :8787 and web app on :5173
- `pnpm dev:desktop`: build the API, then start Electron
- `pnpm lint`: oxlint on the TypeScript (`.oxlintrc.json`, warnings fail), then gofmt and staticcheck on the API (`apps/api/scripts/lint.mjs`; staticcheck is pinned as a Go tool in `go.mod`)
- `pnpm typecheck`: `go vet` and `tsc` (TypeScript 7, the native compiler; typescript-eslint does not support it, hence oxlint)
- `pnpm test`: `go test ./...` and vitest
- `pnpm check`: lint, typecheck and test, as CI runs them
- `pnpm data:sync` / `pnpm data:check`: import or check the game data release
- `pnpm notices`: write `THIRD_PARTY_NOTICES.md` again after changing dependencies (CI fails when it is stale)
- `pnpm build:desktop:mac|win|linux`: local installers in `apps/desktop/release/<version>` (never published)
- Branches: work lands in `develop` (default branch, squash merges); `main` only receives `develop` through a merge-commit pull request before each release.
- `pnpm release X.Y.Z`: bumps every `package.json`, merges it into `develop`, then `develop` into `main` (`scripts/release.mjs`, through `gh`); the maintainer then runs the Release workflow, which always releases `main`: it tags, builds and publishes. Developer reference: `docs/DEVELOPMENT.md`.

## Conventions

- Interface code goes in `packages/app`; the shells only provide the API address, the app version and, on desktop, the update bridge (`Platform.updates`). Never import Electron or Node APIs from `packages/app`.
- Game rules live in Go (`apps/api/internal/engine`): stats, set bonuses, equip conditions, forgemagic, weapon and spell damage. The interface asks `/api/builds/evaluate` and `/api/builds/damage` instead of recomputing them.
- SQLite is the store of record (`internal/store`). Changing a table means bumping `schemaVersion`, which drops the game data and triggers a re-import.
- Every user-facing string has an entry in each language file of `packages/app/src/locales` (French, English, Spanish, Portuguese). French defines the keys, so a missing key fails `pnpm typecheck`; use the game data's own terms for stats, slots and classes in each language, and a `{ one, other }` message where a count changes the words.
- Colours come from the tokens in `packages/app/src/styles.css`; see `DESIGN.md` and `PRODUCT.md` before changing the look.
- The app never asks for an account or personal data, and only goes online for the game data and its own updates (README, PRIVACY.md). Don't add telemetry, logins or other network access.
- Never commit Ankama content (game data, images); the app downloads it at run time. The README screenshots (`docs/screenshots`) are taken without the item icons, so the app shows its slot glyphs instead. Every place the app is described says it is unofficial and not affiliated with Ankama.
- dofusbook (d-bk.net) is the reference for formulas. Compare by reading its rendered pages; never call its private API.
- React: no `setState` in effects to reset state; adjust it during render or in the event handler (oxlint enforces it).
- Before using ui-sdk, read `packages/app/node_modules/@ui-sdk/react/docs/llms.txt` and the page of the component you use.
