# Development

How Dofus Gear Explorer is built: the monorepo, the game data import, the HTTP API, the forgemagic rules and the solver. To contribute, start with [CONTRIBUTING.md](../CONTRIBUTING.md).

## Requirements

- Node.js 22 or newer and pnpm 10
- Go 1.27 or newer

## Getting started

```bash
pnpm install
```

```bash
pnpm dev
```

This starts the API on `http://127.0.0.1:8787` and the web app on `http://localhost:5173`. On first start the API downloads the latest game data release and imports it into SQLite; the app shows the progress.

To run the desktop app instead:

```bash
pnpm dev:desktop
```

### Packaging the desktop app

Releases are built by the `Release` workflow when a `vX.Y.Z` tag is pushed (see CONTRIBUTING.md). To build locally:

```bash
pnpm build:desktop:mac
```

```bash
pnpm build:desktop:win
```

```bash
pnpm build:desktop:linux
```

The installers land in `apps/desktop/release/<version>`. The API is cross-compiled for each target into `apps/api/bin/<os>-<arch>` (it has no C dependency, so any system builds every target) and the package for a platform only carries its own binary.

Local builds never publish (`--publish never`). The macOS app is signed ad hoc (`mac.identity: "-"`), so the package carries no developer name, and it is not notarized. The Windows installer is not signed. Removing the warnings below needs an Apple Developer ID and a Windows code signing certificate. The icon is drawn in `apps/desktop/build/icon.svg`; render it again with `rsvg-convert -w 1024 -h 1024 apps/desktop/build/icon.svg -o apps/desktop/build/icon.png`.

The version shown in the app comes from `apps/desktop/package.json` (desktop) and `apps/web/package.json` (web). Keep every `package.json` on the same version.

### App updates

The desktop app checks this repository's GitHub releases at startup and every six hours with `electron-updater` (`apps/desktop/src/main/updates.ts`). Development runs skip it.

- **Windows and Linux**: the new version downloads in the background; a notice offers to restart, and it installs on the next quit anyway.
- **macOS**: the native updater only installs updates signed with the same Apple Developer ID, and the app is signed ad hoc, so the app only shows a notice with a link to the release page. Signing with a Developer ID would turn on automatic installs.
- No release yet, offline, or a private repository: the check fails silently.

The update feed is the repository the release workflow ran in, written into the app at build time (`app-update.yml`). The `latest*.yml` files and the macOS `.zip` files in a release are what the updater reads; keep them in every release.

## Layout

```
apps/
  api/       Go backend: data import, SQLite, HTTP API, solver
  web/       Web shell (Vite + React)
  desktop/   Desktop shell (Electron), runs the API as a local sidecar
packages/
  app/       The interface, shared by both shells
```

`packages/app` holds every screen, the API client and the local state. A shell only provides the API address: the web shell reads `VITE_API_URL`, the desktop shell starts the Go binary on a free port and passes its URL through the preload script. Adding the hosted web version later means deploying `apps/web` and the API; no interface code changes.

## Sharing stuffs

A stuff travels as a code: base64url JSON of positional arrays (version, name, level, characteristics, scrolls, items, forgemagic, class), written and read in `packages/app/src/lib/share.ts`. The desktop app copies the code alone, since its own address is a local file; the web version copies a link that carries it in `?b=`, which imports the stuff when opened. "Importer" in Mes équipements accepts either (`readShared`). Older codes (versions 1 and 2) still import. No server is involved.

## Game data

Data comes from the JSON files that [doduda](https://github.com/dofusdude/doduda) publishes in the [dofusdude/dofus3-main releases](https://github.com/dofusdude/dofus3-main/releases). The API reads `items.json`, `item_sets.json`, `item_types.json`, `item_super_types.json`, `effects.json`, `characteristics.json`, `chars_categories.json`, `spells.json` and one translation file per language, then writes them to `apps/api/data/dofus.db`. Item icons are extracted to `apps/api/data/images`.

Only equipment is imported: amulets, rings, belts, boots, hats, cloaks, weapons, shields, pets and mounts, Dofus, trophies and prysmaradites. Items no player can equip (game master items, disabled items) and items without any effect are left out.

Three more things come from the same files:

- **Forgemagic weights**: each characteristic's rune weight (`effectPowerRate`), which sets how high a line can be forged.
- **Transcendence runes**: the 81 runes, the bonus each gives and its density (effect 2826: 40, 60 or 80).
- **Passive effects**: Dofus, prysmaradites and legendary items point at a spell whose description is their passive.
- **Classes and spells**: the 19 classes, their spells and variants and the 12 common spells (`breeds.json`, `spell_variants.json`, `spell_levels.json`), with the direct damage lines of each grade.

### Updating

The API checks for a new release at startup and every six hours.

- By default it only **reports** the update: the app shows a banner with the new version and an "Update" button.
- With `--auto-update` (or `DDE_AUTO_UPDATE=true`) it imports new releases by itself.

An import replaces the data in one transaction, so the app keeps serving the previous release until the new one is complete. Saved stuffs reference items by their game id and survive updates.

From the command line:

```bash
pnpm data:check
```

```bash
pnpm data:sync
```

`data:check` prints whether a newer release exists. `data:sync` imports the latest one. The underlying command takes more options:

```bash
cd apps/api && go run ./cmd/dde-api sync --version 3.7.13.17
```

```bash
cd apps/api && go run ./cmd/dde-api sync --source /path/to/doduda/output
```

`--source` imports files from a local directory, such as the output of your own `doduda` run, instead of GitHub.

### API options

| Flag | Environment | Default | Purpose |
| --- | --- | --- | --- |
| `--addr` | `DDE_ADDR` | `127.0.0.1:8787` | Listen address. Port `0` picks a free port. |
| `--data-dir` | `DDE_DATA_DIR` | `data` | SQLite database and icons. |
| `--langs` | `DDE_LANGS` | `fr,en` | Languages to import. |
| `--repo` | `DDE_RELEASE_REPO` | `dofusdude/dofus3-main` | Repository publishing the releases. |
| `--check-interval` | | `6h` | Delay between release checks. |
| `--auto-update` | `DDE_AUTO_UPDATE` | off | Import new releases automatically. |
| `--image-size` | | `128` | Icon size to import, `64` or `128`. Changing it downloads the icons again at the next start. |
| `--cors-origin` | `DDE_CORS_ORIGIN` | `*` | Allowed origin for browsers. |
| `--web-dir` | `DDE_WEB_DIR` | | Serve a built web app from the API. |

Set `GITHUB_TOKEN` if you hit GitHub's anonymous rate limit.

## HTTP API

| Route | Purpose |
| --- | --- |
| `GET /api/data/status` | Installed and latest versions, update progress |
| `POST /api/data/check` | Look for a new release now |
| `POST /api/data/update` | Import the latest release |
| `GET /api/meta` | Characteristics, item types, slots |
| `GET /api/items` | Search equipment: `q`, `slots`, `levelMin`, `levelMax`, `stat=11:gte:300`, `passive=true`, `sort`, `order`; paged with `offset` and `limit` (50 by default, 200 at most), `total` in the answer |
| `GET /api/items/{id}` | One item with its set |
| `GET /api/sets/{id}` | One set with its bonuses |
| `POST /api/builds/evaluate` | Characteristics, set bonuses and validity of a stuff, with the forgemagic in `mods` |
| `POST /api/builds/damage` | What the stuff's weapon and the spells of `breedId` hit for; `range` forces `melee` or `ranged` |
| `POST /api/solver/search` | Stuffs that meet constraints; `forge` says which exos, runes and dracorunes are allowed, `required` lists items every stuff must wear, `excluded` those none may wear, `damage` lists attacks that must hit for a value. Answers the first `limit` stuffs (5 by default, 20 at most) with `total` and an `id` |
| `GET /api/solver/results/{id}` | The next stuffs of a search: `offset`, `limit`. The API keeps the last 8 searches in memory and answers 404 `results_expired` past that |

Every data route takes `lang=fr` or `lang=en`. Answers are gzipped when the client accepts it.

Lists that come from the API are paged by the API: the explorer asks for 60 items at a time as the table scrolls, and the search results for 5 stuffs at a time. The list of stuffs is the exception: stuffs live in the browser's storage, so that table is cut into pages of 50 in the interface.

## Forgemagic

The app does not simulate forging: there are no success odds and no leftover weight. The player states the result they want on an item, and the engine applies it and checks that the game allows it. Amulets, rings, belts, boots, hats, cloaks, weapons and shields can be forged; Dofus, trophies, pets and mounts cannot.

| What | Rule applied |
| --- | --- |
| Roll | Any value between the item's worst and best roll. Best roll by default. |
| Over | A natural line above its best roll, up to 101 of weight (505 Vitalité, 101 Force, 33 Sagesse…), or the item's own best roll when that is higher. |
| Exo | A line the item does not have, up to 101 of weight, so 1 PA, 1 PM or 1 PO. One PA/PM/PO exo per item, and one of each per character. |
| Transcendence rune | One per item, on an item with no over and no exo. The weight of the line it lands on plus the rune's density must stay within 101: 20 Dommages Eau weigh 100, so no Do Eau rune fits. Judged on the lines as they stand, dracorune included. |
| Dracorune (update 3.8) | One per item, also on an over, exo or transcended item. |

Dracorunes follow the 3.8 devblog, since the update is not in the game data yet:

- **Turquoise, Emerald, Crimson, Ochre** turn one elemental characteristic and its damage into Chance/Water, Agility/Air, Intelligence/Fire or Strength/Earth. The element obtained must not already be on the item.
- **Ivory** moves the fixed and % resistances of one element to another the item does not have.
- **Ebony** merges positive elemental characteristics and damage into one Power line and one Damage line, at 70% of the highest value. Negative lines stay.

Assumptions to check when 3.8 ships: the devblog only names Emerald as Agility, so the other three follow the element of their primordial Dofus; the Ebony rune is taken to include Neutral damage and to round down; and the rune's Power and Damage are added to lines the item already has.

Not modelled: the 3.8 set bonus that grants a primordial Dofus's effect. Whether a dracorune may still be applied after a transcendence rune is not known yet; the engine checks the rune against the converted lines either way.

## Damage

A line of a weapon or a spell hits for `floor(base × (100 + characteristic + Power) / 100)` plus Damage and the element's damage (and Critical damage on a critical hit), then the final bonuses: % weapon or spell damage, and % melee or ranged damage. Neutral scales with Strength. A "best element" line follows the highest elemental characteristic. A weapon's critical hit adds its bonus to each line; a spell has its own critical values. An attack counts as melee when it only reaches the next cell, unless the request says otherwise.

Only direct damage is computed: damage and steal lines the game lists in the description, cast as they are. Poisons, triggered effects, heals, conditional bonuses and the target's resistances are left out. When a spell has several lines, each is listed and the lines that hit at once are summed; a spell whose lines hit different targets is overstated by that sum.

A weapon is counted with its mastery (+300 Power) unless the request says `mastery: false`, and with its elemental forgemagic when it has one (`mods.weapon.element`: its Neutral lines hit in that element). An attack also has a critical rate, its own chance plus the character's Critical, capped at 100, and an average hit: normal and critical hits weighed by that rate.

These numbers were checked against dofusbook on a full stuff: stats, spell lines and the weapon's lines, total, critical rate and average are identical.

The search reads one of three measures on that sum, `normal`, `crit` or `mean` (the average hit). Besides a weapon or a spell, a goal can be of kind `element`: the average of the measure over the spells of the class (`breedId`) that hit in that element, with no spell named. Left without an element, it gets a rough search in each of the four, then the full search in the one that came out on top. An entry of `damage` asks that it reaches a value; an entry of `maximize` with a `damage` field pushes it up, at its rank among the stats to maximize. Either way, a weapon is worn by the stuff.

## How the solver works

The search cannot enumerate every stuff. It runs simulated annealing from many random starts and from each shortlisted set and pair of sets, crosses the best stuffs found by exchanging three or four of their pieces at once, tries every swap of two items on the leaders, and re-chooses the leaders' forgemagic as a whole. A request always returns the same result; it is the best found, not a proven optimum.

A stuff has 16 positions with hundreds of candidates each, and set bonuses tie positions together, so the combinations cannot be listed one by one. The solver:

1. keeps, for each slot, the items that matter most for the requested characteristics, plus the pieces of the most promising sets;
2. runs simulated annealing in parallel over those candidates, scoring each stuff on the characteristics to maximize and penalizing missed constraints and broken equip conditions;
3. polishes the best stuffs with a greedy pass and returns distinct results.

What the search maximizes, characteristics and attacks alike, is ranked: each entry counts ten times less than the one before it in the list, so a lower priority decides between stuffs that are close on the higher ones. Characteristics are compared on the scale of forgemagic rune weights, so one PA weighs as much as about a hundred Force. A typical search evaluates several million stuffs in well under a second. When nothing satisfies every condition, the closest stuffs are returned with the missed conditions marked.

When forgemagic is allowed in a search:

- an allowed PA, PM or PO exo counts as one more point, then goes to an item of the result that lacks the stat;
- each forgeable item comes with every forgemagic the request allows (each dracorune, the strongest rune that fits per useful characteristic, a negative Critical line forged up to -1), and the search switches between them on the stuff's real score: a rune that fills a missed condition beats one that adds to what is maximized;
- an exo needs an item with no transcendence rune, so the search keeps one free per exo.

Required items take the first free place that fits them, before the items kept from the current stuff; one that finds no place is reported in `unplaced`.

Overs are left to the player: the solver never assumes them.

By default the search also spends the character's characteristic points and counts every characteristic as scrolled to 100. Points follow the game's prices: Vitalité 1 for 1, Sagesse 3 for 1, and Force, Intelligence, Chance and Agilité at 1, 2, 3 then 4 points per point for each hundred already bought. For every stuff it looks at, the solver first buys what the conditions still miss, then puts the rest where it serves the request best; points nothing asked for go to Vitalité. Opening a result in the builder carries the same points and scrolls. Both options can be turned off to keep the active stuff's values.

Every result fills all 16 positions: when the request is served, the remaining slots take the strongest items left. A slot stays empty only if every item for it would make the stuff invalid. The stuff's level is a ceiling only: items above it are left out, and a lower-level item is worn whenever it serves the request better.

PA, PM and PO are capped at 12, 6 and 6, as in the game. A stuff may add up to more and stays valid; the points above the cap are lost, the builder says how many, and the solver gains nothing by going over.

A prysmaradite only goes in the first Dofus position, so a stuff holds at most one. The builder, the item picker and the solver all apply it.

The stat engine uses the best roll of each item unless forgemagic says otherwise, adds set bonuses, and applies the bonuses characteristics give each other (initiative, prospecting, tackle, dodge, pods). Passive effects are shown but not counted: they depend on what happens in a fight.

## Scripts

| Command | What it does |
| --- | --- |
| `pnpm dev` | API and web app |
| `pnpm dev:desktop` | Build the API, then start the Electron app |
| `pnpm build` | Build every package |
| `pnpm build:desktop` | Package the desktop app with the API inside, for the system you are on (builds the API for every target) |
| `pnpm build:desktop:mac` | macOS disk images, Apple silicon and Intel |
| `pnpm build:desktop:win` | Windows x64 installer; works from macOS too |
| `pnpm build:desktop:linux` | Linux x64 AppImage |
| `pnpm notices` | Write `THIRD_PARTY_NOTICES.md` again after a dependency change (CI checks it) |
| `pnpm lint` | oxlint on the TypeScript, gofmt and staticcheck on the Go API |
| `pnpm typecheck` | `go vet` and `tsc` |
| `pnpm test` | Go and TypeScript tests |
| `pnpm check` | Lint, typecheck and test, as CI runs them |
| `pnpm data:sync` | Import the latest game data |
| `pnpm data:check` | Report whether an update exists |
