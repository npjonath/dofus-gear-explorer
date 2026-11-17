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

Only the maintainer publishes releases: the Release workflow refuses to run for anyone else, and release tags can't be moved or deleted.

1. Run `pnpm release X.Y.Z` (it needs the GitHub CLI, `gh`, signed in). It sets the version in every `package.json` on a `release/vX.Y.Z` branch and merges that pull request into `develop` once CI passes. Then it opens a pull request from `develop` to `main` and merges it with a merge commit once its checks pass. Both pull requests carry `skip-changelog`. If a check fails, fix it and run the command again: it picks up where it stopped.
2. On GitHub, open Actions, then **Release**, then **Run workflow**, and keep the `main` branch. The workflow checks the version, runs the checks, builds the installers for macOS, Windows and Linux, and publishes them in a release tagged `vX.Y.Z` with generated notes. Installed apps then update to it.

If a packaging job fails (for example, no macOS runner is free), use **Re-run failed jobs**: the release is published once every installer is attached. Tick **Dry run** to only build the installers, as workflow artifacts, from any branch.

The release notes group the merged pull requests by label: `enhancement` under New features, `bug` under Fixes, `dependencies` under Dependencies. `skip-changelog` leaves a pull request out.
