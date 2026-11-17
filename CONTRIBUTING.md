# Contributing to Dofus Gear Explorer

Thanks for helping. Dofus Gear Explorer is a free, open-source tool made by players for players, and every fix, idea or translation counts.

## Ways to help

- **Report a bug or a wrong number.** Open an issue with the stuff's code (the share button of the stuff), what the app shows and what the game or dofusbook shows.
- **Ask a question or float an idea.** Use [Discussions](../../discussions).
- **Suggest a feature.** Open an issue and describe what you are trying to do in the game, not only the button you want.
- **Send code.** Small pull requests are reviewed fastest. For anything large, open an issue first so we agree on the approach.

## Licence of contributions

The project is under the [MIT licence](LICENSE). By opening a pull request you agree that your contribution is published under that same licence, and you confirm that you wrote it or have the right to submit it. There is no contributor licence agreement to sign.

## Ground rules

- **No Ankama content in the repository.** Do not commit game data, item images, sounds or any other file taken from the game or from Ankama's sites. The app downloads the game data at run time from the [doduda](https://github.com/dofusdude/doduda) releases.
- **No private APIs.** Do not call the private APIs of other sites (dofusbook and the like). When a formula has to be checked, compare with what their pages display.
- **No accounts, no tracking.** The app never asks for a Dofus account or personal information, and its only network use is downloading game data and its own updates. Pull requests that add analytics, telemetry, ads or a login will not be merged.
- **Every language.** Every string the user sees has an entry in each of `packages/app/src/locales/fr.ts`, `en.ts`, `es.ts` and `pt.ts`. A missing key fails `pnpm typecheck`. If you don't speak one of them, say so in the pull request and someone will check the wording.
- **Game rules live in Go.** Stats, set bonuses, conditions, forgemagic and damage are computed in `apps/api/internal/engine`; the interface asks the API instead of recomputing them.
- Be kind. The [code of conduct](CODE_OF_CONDUCT.md) applies everywhere in the project.

## Development

You need Node 22 or newer, pnpm 10 and Go 1.27.

```bash
pnpm install
```

```bash
pnpm dev
```

This starts the API on `http://127.0.0.1:8787` and the web app on `http://localhost:5173`. The first start downloads and imports the game data (a few minutes).

Before opening a pull request, run what CI runs (lint, typecheck and tests):

```bash
pnpm check
```

Each step also runs on its own: `pnpm lint` (oxlint for TypeScript, gofmt and staticcheck for Go), `pnpm typecheck`, `pnpm test`.

The architecture, the HTTP API, the forgemagic rules and the solver are described in [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Pull requests

- `develop` is the development branch and the default one: branch from it and open your pull request against it. `main` only holds released versions.
- CI (lint, typecheck, tests, build, and the API tests on Windows) and CodeQL must pass. Only the maintainer merges pull requests.
- Pull requests are squash-merged into `develop`. Give the pull request a label (`bug`, `enhancement`, `dependencies`) so it lands in the right section of the release notes.
- Dependabot opens weekly pull requests for GitHub Actions, Go modules and npm packages, a week after each version is published. Actions are pinned to a commit SHA with the version in a comment; Dependabot keeps both in step.

## Releases

Only the maintainer publishes releases: `v*` tags can only be created by a repository admin.

1. Set the new version in every `package.json` (`node scripts/check-version.mjs X.Y.Z` confirms they agree) and merge that change into `develop`.
2. Open a pull request from `develop` to `main`, label it `skip-changelog`, and merge it with a merge commit (the only method `main` accepts, so both branches keep the same history).
3. Optionally run the **Release** workflow by hand (Actions tab, "Run workflow"): it builds the installers for macOS, Windows and Linux as workflow artifacts without publishing anything.
4. Push a tag `vX.Y.Z` on `main`. The workflow checks the version, runs the checks, builds the installers and attaches them, with generated notes, to a draft GitHub release.
5. Read the draft, then publish it. Installed apps pick up the new version from the published release; a draft is invisible to them.
