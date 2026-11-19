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

Development runs keep their stuffs, settings and game data in their own folder, `Dofus Gear Explorer Dev` (next to the installed app's `Dofus Gear Explorer`), so they never share storage with an installed copy. Set `DDE_DATA_DIR` to reuse game data you already imported, for example `DDE_DATA_DIR=$PWD/apps/api/data pnpm dev:desktop`. Only one copy of the app runs at a time: opening it again brings back the window already open.

### Packaging the desktop app

Releases are built and published by the `Release` workflow, run by hand after `pnpm release X.Y.Z` (it always releases `main`) (see CONTRIBUTING.md). To build locally:

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

The desktop app checks this repository's GitHub releases with `electron-updater` (`apps/desktop/src/main/updates.ts`): at startup, every hour, when its window comes back to the front or the computer wakes after an hour without a check (timers stand still while it sleeps), and when the player picks "Check for updates" in the About menu. Development runs skip it.

On every system the new version downloads in the background; a notice offers to restart, and it installs on the next quit anyway. The main process tells the interface each step through `Platform.updates` (`downloading`, then `ready`, or `available` when the app can only link to the release page). The interface shows it as a button in the top strip next to the About button (`packages/app/src/shell/UpdateButton.tsx`), which stays after the notice is dismissed.

- **Windows and Linux**: `electron-updater` downloads and installs it (the NSIS installer, the AppImage). On Linux only the AppImage updates itself.
- **macOS**: the native updater (Squirrel.Mac) only installs updates signed with the same Apple Developer ID, and the app is signed ad hoc. So `electron-updater` only finds the release, and `apps/desktop/src/main/macUpdate.ts` installs it. It downloads the release's `-mac-<arch>.zip`, checks it against the sha512 in `latest-mac.yml`, and unpacks it with `ditto`. Then it checks the bundle id, the version and the code signature (`codesign --verify --deep --strict`). Once the app quits, a small script swaps the `.app` bundle and puts the old one back if the move fails. When the bundle can't be replaced (the app runs from the disk image, from a read-only folder, or from the copy Gatekeeper makes of an app never moved to Applications), the notice links to the release page instead.
- No release yet, offline, or a private repository: the check fails silently.

The update feed is the repository the release workflow ran in, written into the app at build time (`app-update.yml`). The `latest*.yml` files and the macOS `.zip` files in a release are what the updater reads; keep them in every release.

The **Update test** workflow checks a real update on Windows, Linux and macOS whenever `apps/desktop` changes. `apps/desktop/scripts/update-test.mjs` packages the app as 1.0.0 and 1.0.1, both pointing at a feed served on the machine. It installs 1.0.0 (the NSIS installer silently, the AppImage, the macOS zip), starts it with remote debugging, and waits for the update to be ready. Then it presses restart through the same bridge as the notice and checks that 1.0.1 is installed. To run it locally, build the API for your system (`pnpm --filter @dde/api build:targets mac-arm64`) and the interface (`pnpm --filter @dde/desktop exec electron-vite build`), quit the installed app, then run `node scripts/update-test.mjs` from `apps/desktop`.

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

Data comes from the JSON files that [doduda](https://github.com/dofusdude/doduda) publishes in the [dofusdude/dofus3-main releases](https://github.com/dofusdude/dofus3-main/releases). The API reads `items.json`, `item_sets.json`, `item_types.json`, `item_super_types.json`, `effects.json`, `characteristics.json`, `chars_categories.json`, `spells.json`, `spell_types.json` and one translation file per language, then writes them to `apps/api/data/dofus.db`. Item icons are extracted to `apps/api/data/images/items`, and the fight simulator's spell icons and class portraits (`spell_images_96.tar.gz`, `class_head_images_64.tar.gz`) to `images/spells` and `images/breeds`; a release without them leaves the app with initials.

Only equipment is imported: amulets, rings, belts, boots, hats, cloaks, weapons, shields, pets and mounts, Dofus, trophies and prysmaradites. Items no player can equip (game master items, disabled items) and items without any effect are left out.

Three more things come from the same files:

- **Forgemagic weights**: each characteristic's rune weight (`effectPowerRate`), which sets how high a line can be forged.
- **Transcendence runes**: the 81 runes, the bonus each gives and its density (effect 2826: 40, 60 or 80).
- **Passive effects**: Dofus, prysmaradites and legendary items point at a spell whose description is their passive.
- **Classes and spells**: the 19 classes, their spells and variants and the 12 common spells (`breeds.json`, `spell_variants.json`, `spell_levels.json`), with the direct damage lines of each grade. For the fight simulator, each grade also keeps how it is cast and every effect as the game lists it, and the spells a castable spell casts in turn are imported with it (filed under no class, never listed).

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
| `--langs` | `DDE_LANGS` | `fr,en,es,pt` | Languages to import. A language missing from the installed data is imported again from the same release at the next start. |
| `--repo` | `DDE_RELEASE_REPO` | `dofusdude/dofus3-main` | Repository publishing the releases. |
| `--check-interval` | | `6h` | Delay between release checks. |
| `--auto-update` | `DDE_AUTO_UPDATE` | off | Import new releases automatically. |
| `--image-size` | | `128` | Icon size to import, `64` or `128`. Changing it downloads the icons again at the next start. |
| `--cors-origin` | `DDE_CORS_ORIGIN` | `http://localhost:5173,http://127.0.0.1:5173` | Browser origins allowed to call the API, comma separated. `*` allows any. |
| `--web-dir` | `DDE_WEB_DIR` | | Serve a built web app from the API. |

Set `GITHUB_TOKEN` if you hit GitHub's anonymous rate limit.

The API runs on the player's computer, where any web page open in their browser could send it requests. It answers only:

- requests without an `Origin` header (the desktop app's `file://` pages send none, nor do tools like `curl`);
- requests from its own pages (`--web-dir`) or from an origin listed in `--cors-origin`; every other page gets `403 forbidden_origin`.

When it listens on a loopback address (the default), it also rejects requests addressed to any other host name (`403 forbidden_host`), so a page cannot rebind its own domain to `127.0.0.1` to pass as same-origin. If you serve the web app from another address, add its origin to `--cors-origin`.

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
| `POST /api/builds/damage` | What the stuff's weapon and the spells of `breedId` do: each line with its targets, whether it counts in the total, and heals and pushbacks; `range` forces `melee` or `ranged` |
| `POST /api/fight` | The fight simulator: without `state`, a new fight against `dummies` for the stuff; with `state` and an `action` (`place`, `begin`, `move`, `cast`, `end`), the fight after that action, with what happened, the points left, the cells in reach and each spell's availability |
| `POST /api/fight/aim` | Aimable cells with the area, normal and critical damage ranges, and predicted displacements. A cast projected through an Eliotrope portal includes the projected cell. |
| `GET /api/fight/book?breed=ID` | Class and common spells with translated descriptions, variant IDs, unlock levels and tags for the spellbook filters. Breed zero returns common spells. |
| `GET /api/fight/assets` | Training room IDs and the identifier of the currently imported local pack |
| `POST /api/solver/search` | Stuffs that meet constraints; `forge` says which exos, runes and dracorunes are allowed, `required` lists items every stuff must wear, `excluded` those none may wear, `damage` lists attacks that must hit for a value, and an entry of `maximize` can be an attack too, over several elements if need be. Answers the first `limit` stuffs (5 by default, 20 at most) with `total` and an `id` |
| `GET /api/solver/results/{id}` | The next stuffs of a search: `offset`, `limit`. The API keeps the last 8 searches in memory and answers 404 `results_expired` past that |

Every data route takes `lang=fr` or `lang=en`. Answers are gzipped when the client accepts it.

Lists that come from the API are paged by the API: the explorer asks for 60 items at a time as the table scrolls, and the search results for 5 stuffs at a time. The list of stuffs is the exception: stuffs live in the browser's storage, so that table is cut into pages of 50 in the interface.

## Forgemagic

The app does not simulate forging: there are no success odds and no leftover weight. The player states the result they want on an item, and the engine applies it and checks that the game allows it. Amulets, rings, belts, boots, hats, cloaks, weapons and shields can be forged; Dofus, trophies, pets and mounts cannot.

| What | Rule applied |
| --- | --- |
| Roll | Any value between the item's worst and best roll. Best roll by default. |
| Over | A natural line above its best roll, up to 101 of weight (505 Vitalité, 101 Force, 33 Sagesse…), or the item's own best roll when that is higher. |
| Exo | A line the item does not have, up to 101 of weight, so 1 PA, 1 PM or 1 PO. One PA/PM/PO exo per item. A character counts one exo of each: a second one leaves the stuff valid, as the game lets both items be worn, but adds nothing (`ignoredExos` in the evaluation). |
| Shared weight | The overs and exos of an item share 101 of weight (`forge_weight` when they pass it). An over counts for what it adds above the best roll, an exo for its whole line: an MP exo (90) leaves room for 11 Force or 55 Vitalité of over, an AP exo (100) for 1 of weight. |
| Heavy lines | When a rune leaves a line in over, the game counts that whole line plus the overs and exos already on the others, within 101. The engine does not know the order of the runes, so it accepts a result when one order passes: overs from the heaviest natural line to the lightest, then exos. Two overs on lines that are already near 101 fail in any order. |
| Malus | A negative line can be forged away, up to 0 and no further. That is an over. A rune weighs half on a negative line, and its full weight for the last point, from -1 to 0: raising -5 Critique to -1 weighs 20 (4 × 10 / 2) and to 0 weighs 30, which leaves no room for an MP exo. Taking a malus off a PA, PM or PO line counts as that exo for the character. |
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

Each line of a spell says who it applies to, from the game's target mask (`a,A` allies and enemies, `A` enemies, `j,J` summons, `g,A` everyone but the caster, and so on): the same spell can hit summons harder, or heal allies and hurt enemies. A line can also be *conditional*: the spell only describes it, and it applies later, when the state the spell leaves on its target is triggered (Éboulement's second hit, when the target loses Range). The game flags those lines as described only; when every damage line of a spell is flagged, they are the spell's hit, dealt through another spell.

The total and the average of an attack count what an enemy that is not a summon takes as the attack is made (`AttackLine.Hits`): not the lines kept for summons or allies, the conditional or delayed ones, nor what is not damage. The search reads the same lines.

Besides damage, the panel computes:

- a heal that scales with a characteristic: `floor(base × (100 + characteristic) / 100) + Soins`, without Puissance;
- a heal in percent of the caster's health points;
- pushback damage: `floor((level / 2 + 32 + Dommages Poussée) × cells / 4)`;
- what a steal gives back: half the damage;
- a line whose base grows with each MP the caster has used this turn (Tourbière), for one MP up to the character's own;
- a line in the caster's lowest characteristic (Flamèche), as there is one in the highest.

Poisons, glyphs, traps and what a summon casts are not computed. All of the above was checked against dofusbook on an Enutrof stuff: the 30 stats, and 42 of the 43 spells it lists (the last one heals through a summon).

The target has no resistance. Only the lines the game lists in a spell's description are read, cast as they are.

A weapon is counted with its mastery (+300 Power) unless the request says `mastery: false`, and with its elemental forgemagic when it has one (`mods.weapon.element`: its Neutral lines hit in that element). An attack also has a critical rate, its own chance plus the character's Critical, capped at 100, and an average hit: normal and critical hits weighed by that rate.

These numbers were checked against dofusbook on a full stuff: stats, spell lines and the weapon's lines, total, critical rate and average are identical.

The search reads one of three measures on that sum, `normal`, `crit` or `mean` (the average hit). Besides a weapon or a spell, a goal can be of kind `element`: the average of the measure over the spells of the class (`breedId`) that hit in that element, with no spell named. Left without an element, it gets a rough search in each of the four, then the full search in the one that came out on top. An entry of `damage` asks that it reaches a value; an entry of `maximize` with a `damage` field pushes it up, at its rank among the stats to maximize. Either way, a weapon is worn by the stuff.

## Fight simulator

The Fight view of a stuff plays turns against training dummies (`apps/api/internal/fight`). The server keeps nothing: the interface holds the fight and sends it back with each action, so a fight is plain data and every step is a pure function of it.

- **Board**: the imported Village and Astrub Kanojedos use their own movement cells, sight obstacles and starting camps. Without an art pack the tactical arena uses a 13 × 13 grid with four pillars. Range counts cells along rows and columns. A cell is in sight when the straight line to it crosses no wall and no entity; a line through the exact corner of four cells crosses none.
- **Casting**: a spell grade's flags in `spell_levels.json` (`m_flags`: 1 in line, 2 diagonal, 4 needs sight, 8 a free cell, 16 a taken cell, 64 range follows the caster's Range) and its `maxCastPerTurn`, `maxCastPerTarget`, `minCastInterval`, `initialCooldown` and `maxStack`. The weapon is cast in sight, at one cell, within its range.
- **Effects** are applied in the order of the data. Who an effect is meant for is read from its zone and its target mask as things stood when the spell was cast: where everyone stood, and the states everyone was in. So a push does not make the next effect miss, and a spell that puts its caster in a state and has another effect for casters in that state applies one of the two. A mask names who (`a` allies, `A` enemies, `C` the caster wherever it stands, `g` allies but the caster) then conditions that must all hold: `E`/`e` in a state or not, `V`/`v` at or under a share of its health or above, `B`/`b` of a class or not, starred when asked of the caster. The other letters ask what a player or a dummy never is (a given monster, a summon, carried): the upper-case one fails, its lower-case opposite passes. An effect kept for monster 50000 is how the data shows a line in a spell's description without applying it.
- **Spells cast in turn**: most class spells work through others. Actions 792, 1017 and 1018 have the target cast one on its own cell, 1019 on the source, 1160 has the caster cast it on the target's cell, 2160 the source, 2794 the target on the originally aimed cell and 2960 the caster on that cell whoever stands there.
- **Effects that wait**: an effect with a trigger stays on its target and applies each time the trigger happens (`TB` and `TE` the start and end of its turn, `EON<id>` and `EOFF<id>` a state gained or lost, `CC` a critical cast, `D` a hit taken, with `DM`/`DR` from the next cell or from afar, `DE`/`DF`/`DW`/`DA`/`DN` by element, `DBE`/`DBA` from an enemy or an ally, `H` a heal received) for `duration` turns, or `effectTriggerDuration` when the duration is zero (63 is the whole fight). An effect with a `delay` counts for nothing until that many turns have passed: without a trigger it then applies once, with one it starts waiting. A compound trigger such as `I|TB` applies on cast and at subsequent starts of turn. Mutilation uses this sequence for its current-HP cost. Damage with a trigger and a duration is a poison.
- **What opens a fight**: each class has starting spells, filed in the data under a spell type named after it ("Sorts initiaux Pandawa", "Pandawa Starting Spells") with the admin name "Sort initial"; they are cast on the character as the fight starts and set its states and what waits on it (a Pandawa starts sober, a Forgelance armed). So is the first grade of the passive of each item worn (an item's grades are its phases: the Ochre Dofus's second grade is the state a hit leaves, its third what a turn's start gives).
- **Lasting effects**: characteristics raised or lowered, states, damage taken or heals received multiplied, and what changes one spell: its base damage, range, cost, critical chance, casts per turn or per target, cooldown and aiming rules (actions 280 to 299, 314, 2905, 2906). Final damage in percent (107), Power for spells or for weapons (98, 103) and resistance in percent (101) are read when a hit is dealt.
- **Random outcomes**: effects with a `random` weight are drawn as one lottery per spell; effects sharing a `group` happen together, and the weights are chances in percent. The preview gives the least and the most over every outcome.
- **Damage**: the formula of the Damage section, with melee decided by the distance when the spell was cast; then the hit loses 10% per cell away from the centre of a zone (the zone's own figures), then the target's fixed resistance comes off, then its percentage. A push stopped short deals `floor((level / 2 + 32 + Pushback damage) × cells left / 4)`, and half of it to whoever blocks the way.
- **Eliotrope portals** use the same route for highlighting, movement and spell projection. Each side keeps four gates in placement order. A new gate displaces the oldest. Walking through a gate spends the step to its entrance, closes the two ends for the current turn and lands the fighter at the exit. Projection translates the vector from the caster to the entrance onto the exit. The entry grade supplies the flat bonus and bonus per cell of the complete route; a projected cast can then grant the Eliotrope’s final damage and healing buffs. The optional local replay pins both Flamèche ranges on the tested Astrub network.
- **Additional spell effects.** `additional_effects.go` keeps percentage-of-current-HP costs, temporary maximum HP and damage based on missing or eroded HP separate from ordinary elemental hits. It also resolves heals based on damage just inflicted, characteristic theft, forced minimum or maximum rolls, and MP-dependent damage. A fighter keeps the cell where its turn began and a bounded history of previous cells for rewind effects.
- **Directional zones.** Martelo's two arms point back towards the caster. Forks extend through the aimed cell and rectangles have independent width and length. The engine uses an explicit orientation when the effect specifies one. Effects with a list of native map cells use the room's imported grid origin. The aiming preview and resolution use the same cells.
- **Invisibility.** The engine records the invisibility duration separately from the MP buff. A direct hit reveals the attacker. An empty cast or a trap placement can leave them invisible. Revelation also has its own effect and log event.
- **Ground effects and creatures**: glyphs, traps and runes retain their cell masks and execute the referenced spell when a fighter enters, starts or ends a turn, as specified by the effect. The local art import also links creature grades, starting spells, AP, MP, appearances and spell bars. Controllable summons take a turn before the dummies. Carrying and throwing retain the passenger’s identity on the shared tile. Symmetric teleports and telefrag target masks use the positions preceding the effect. Fixed reductions and shields are applied before health loss. Tackle, weapon areas and some class-specific effects still have incomplete coverage. Unsupported visible actions are recorded in the combat log.
- **Dummies** do nothing on their turn and stand back up at full health when defeated. Their resistances are characteristics like any other (33 to 37 in percent, 54 to 58 fixed, 87 critical).

Checked, on the Enutrof of the Damage section:

- the preview of each Enutrof spell on a dummy without resistance gives dofusbook's figures (Roulage de Pelle 364 to 417, 597 to 663 critical; Coup de Grisou 510 to 562 at the centre and 80% of it two cells away);
- against DoPoutch with the same stuff: pushback damage (55 for one cell left, 111 for two at level 200 with 90 Pushback damage), and what the stuff's item passives do turn after turn (Prynyang, Dofus Nébuleux and Dofus Ocre: +30% final damage and -10% resistance on the first turn, -7% and +3% and one more AP on the second), down to Flamèche on the second turn, 86 to 95 and 254 to 264 critical on both sides;
- every class spell was cast once on a dummy and its preview compared with the Damage view's total: 401 of 836 spells give the same figures, 27 differ by the falloff of a zone, 361 deal no direct damage on either side, and 12 deal none in the fight where the Damage view lists some (summons, runes, poisons). The 33 others differ because the Damage view adds lines that are alternatives (a Pandawa's sober and drunk lines, a hit that grows with a state) where the fight applies the one that holds.

### Reference replays

The HTTP replays use a level-150 SPEEDRUSH build, DOFUS 3.7.14.18 data and locally imported rooms. They check the following observations from DoPoutch through the same endpoints the app uses. Damage rolls stay random. A cast assertion uses the normal or critical reference according to the returned cast event.

| Scenario | Observations exercised |
|---|---|
| Ecaflip in the Village | Sword name and cast, summoned Chaton Enragé at 528 HP, 6 AP and 4 MP, then the third-turn Pile ou Face preview of 278 normal or 373 critical. |
| Eliotrope in Astrub | Portals on cells 106 and 134 project Flamèche onto cell 124. That route spans five cells for a 10% bonus. The replay also checks the four-portal limit and the MP cost of stepping through a portal. |
| Masqueraider in Astrub | With the caster on cell 63, aiming Martelo at empty cell 73 previews 216–232 normal or 286–309 critical on cell 62. Aiming directly at 62 gives 271–290 or 358–387. |
| Sram in Astrub | Invisibilité leaves 10 AP and 7 MP, with one turn of invisibility and two turns of extra MP. Casting Flamèche into an empty cell and placing Piège Sournois preserve invisibility. A direct hit reveals the Sram. |
| Sacrier in Astrub | Mutilation grants 150 Power. The reference HP sequence is 2,210 before casting, 1,989 on cast and 1,791 at the start of the next turn. |
| Iop in Astrub | Vitalité takes both current and maximum HP from 2,210 to 2,563 on a normal cast or 2,607 on a critical cast. |

Run the API with the matching data directory and set its loopback address for the tests.

```sh
DDE_FIGHT_TEST_URL=http://127.0.0.1:8787 go -C apps/api test ./internal/httpapi -run 'TestSpeedrush.*WithInstalledGameData' -count=1 -v
```

### Importing combat graphics and sound

In **Combat → Options → General**, the desktop importer can detect DOFUS 3 or open a directory chooser. It reads the installation and writes under the local API's `fight-assets/` directory. For web development, run the same import from the repository root with uv and Python 3.13.

```sh
pnpm data:fight-assets --source "/path/to/DOFUS" --output apps/api/data/fight-assets
```

The pack contains the available room geometry and artwork, actor skins, interface textures and fonts, spell sequences and combat sounds. The sound importer follows the maps' playlists, animation events and interface event IDs into the client's FEV/FSB5 banks and remuxes supported Vorbis samples as Ogg. Its `audio/index.json` identifies the available channels. Music and ambience follow the selected room. Animation and interface cues use the same local pack during playback. One 3.7.14.18 import occupied 2.1 GB and included 3,327 Ogg files (64 MB) for 2,065 sound events across five channels.

An import writes into a staging directory and switches the `current` pointer once the pack is complete. Reimporting reuses an existing generation when its extraction format, client catalogs, audio bank signatures and local skin overrides match. The importer keeps the previous generation for reads already in progress. An optional local `fight-assets/equipment-skins.json` can associate an item ID with a skin ID when the client omits that link. The equipment still contributes its stats when a skin is unavailable.

The installer ships the importer and its dependency licences. The player's local pack holds all extracted artwork, fonts, animations and sounds. Native packaging builds a missing importer with uv. Release CI builds and self-tests separate artifacts for Windows x64, Linux x64, macOS Intel and macOS Apple Silicon. For cross-packaging, unpack the matching artifact under `apps/desktop/fight-import/`. The target directory names are `win-x64`, `linux-x64`, `mac-x64` and `mac-arm64`. To verify an unpacked artifact, run its `fight-import --self-test` executable (`fight-import.exe --self-test` on Windows).

## How the solver works

The search cannot enumerate every stuff. It runs simulated annealing from many random starts and from each shortlisted set and pair of sets, crosses the best stuffs found by exchanging three or four of their pieces at once, tries every swap of two items on the leaders, and re-chooses the leaders' forgemagic as a whole. A request always returns the same result; it is the best found, not a proven optimum.

A stuff has 16 positions with hundreds of candidates each, and set bonuses tie positions together, so the combinations cannot be listed one by one. The solver:

1. keeps, for each slot, the items that matter most for the requested characteristics, plus the pieces of the most promising sets;
2. runs simulated annealing in parallel over those candidates, scoring each stuff on the characteristics to maximize and penalizing missed constraints and broken equip conditions;
3. polishes the best stuffs with a greedy pass and returns distinct results.

What the search maximizes, characteristics and attacks alike, is ranked: each entry counts ten times less than the one before it in the list, so a lower priority decides between stuffs that are close on the higher ones. Characteristics are compared on the scale of forgemagic rune weights, so one PA weighs as much as about a hundred Force. A typical search evaluates several million stuffs in well under a second. When nothing satisfies every condition, the closest stuffs are returned with the missed conditions marked.

A damage goal can cover several elements (`kind: "elements"`, with `parts`, the main element first, and `mix`). Each part reads the class's spells in its element, where a "best element" line counts in the main one only. A part can instead name one attack (`kind: "spell"` or `"weapon"`, with `id`), read on its lines in the element: the average over every spell of a class says little about how the player hits, since a class is often stronger in one element (a Cra's Earth spells against its Water ones). A weapon named by a part is worn by the stuff, its Neutral damage forged to the element of the first part that names it; the parts of a goal name one weapon at most. A goal can hold a single part: one element read on a spell or on the weapon. `GET /api/meta` lists the elements of each spell. Two mixes:

- `floor`: the main element is pushed as high as possible while each other element reaches its `share` (1 to 100) of it; 100 everywhere is a balance. The score is the lowest of the elements once each is divided by its share, plus a small part of their average so the search has a direction while one lags.
- `sum`: the average of the elements weighted by their shares, which is what a turn casting them in those proportions deals. The share an element ends up with is not set by its weight: it depends on what the gear offers.

Under a floor, the characteristic points go to the element furthest behind, ten at a time, so the elements stay level. That split uses an estimate of each element's damage that leaves out the rounding of the formula; the damage that is scored and reported is always the full computation. A solution lists what each element measures in `damageParts`. An "elements" goal can only be maximized, and a request holds one floor at most.

When forgemagic is allowed in a search:

- an allowed PA, PM or PO exo counts as one more point, then goes to an item of the result that lacks the stat;
- each forgeable item comes with every forgemagic the request allows (each dracorune, the strongest rune that fits per useful characteristic, a negative Critical line forged away), and the search switches between them on the stuff's real score: a rune that fills a missed condition beats one that adds to what is maximized;
- an exo needs an item of its own that lacks the stat, holds no transcendence rune and has the weight left for it, so the search keeps one such item per exo: an item that already gives PM is no home for the PM exo.

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
