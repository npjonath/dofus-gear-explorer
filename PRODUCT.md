# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

User-specified: pnpm monorepo, React + Vite, Electron desktop shell, Go backend. A web version ships later from the same UI code, so the interface lives in a shared package (`packages/app`) consumed by `apps/web` and `apps/desktop`. Recommended by the user: TanStack Query, TanStack Table, ui-sdk (vercel-labs/ui). Game data is stored in SQLite. Open source under the MIT licence on GitHub (user decision, 2026-11-17), with CI and releases on GitHub Actions (lint with oxlint and staticcheck, typecheck, tests) and desktop updates from the GitHub releases.

## Users

Dofus players who theorycraft their gear: they compare equipment, assemble a full gear set ("stuff") for a character, and look for the best combination that reaches the stats they want. They already know the game's vocabulary (PA, PM, PO, Sagesse, panoplie, Dofus, trophées) and are used to dofusbook.net.

## Product Purpose

An equivalent of dofusbook.net built on the real, current game data, with a modern interface. It lets a player:

1. Explore every piece of equipment in the game (equipment only, not resources or consumables; classes and spells are imported for damage only).
2. Build a stuff slot by slot and read its resulting characteristics, in a layout familiar from dofusbook.
3. Ask for every stuff that satisfies conditions they set (Sagesse > X, Force > Y, …) and get the best combinations back.

Success: the data matches the live game after each patch, and a player finds a valid stuff for their criteria faster than by hand.

## Positioning

Data comes straight from the game files, as published by doduda in the dofusdude/dofus3-main GitHub releases, not from hand-maintained tables. The app tracks releases, tells the user when a newer one exists, and re-imports it. The stuff search is a solver that works across all 16 equipment slots and set bonuses, rather than a filter on single items.

## Operating Context

- Desktop app (Electron) first; a web version later from the same code.
- The Go backend runs locally beside the desktop app and holds the SQLite database.
- Game data updates arrive as new GitHub releases after each game patch.
- Interface and item names in French and English (user answer: "fr / en"), then Spanish and Portuguese, the other languages the game data is published in (2026-11-17).
- Distributed as installers for macOS (Apple silicon and Intel), Windows and Linux on GitHub Releases. Not signed with paid certificates: the system warns on first opening. Updates install themselves on every system; on macOS the app replaces its own bundle, since the native updater needs a paid signature.
- A community project (user decision, 2026-11-17): issues and pull requests are welcome, contributions are published under MIT, and the README, CONTRIBUTING.md and docs/DEVELOPMENT.md explain how to take part.

## Capabilities and Constraints

- Equipment only: amulets, rings, belts, boots, hats, cloaks, weapons, shields, pets and mounts, Dofus, trophies, prysmaradites.
- Data is imported into SQLite and can be updated from one release to the next; the app can warn that an update is needed (and optionally apply it automatically).
- A stuff has 16 positions: amulet, hat, 2 rings, weapon, shield, belt, cloak, boots, pet, 6 Dofus/trophies.
- Stats use best rolls of each item; set bonuses and equip conditions are taken into account.
- Forgemagic (user request, 2026-11-16): the player can put overs, exos, a transcendence rune and a dracorune on the items of a stuff to see what it would give, in the builder and in the search. It is explicitly not a forgemagic simulator: no success odds, no leftover weight.
- Dracorunes from update 3.8 are included before the update is released, from the devblog's rules.
- Transcendence runes follow the game's rule (user rule, 2026-11-16): an item with no over and no exo, and the weight of the line plus the density of the rune (40, 60 or 80, read from the game data) within 101. The rule is judged on the lines as they stand, dracorune included. The 101 threshold is the user's; one guide says 100.
- Passive effects of Dofus, prysmaradites and legendary items are displayed (user request); they are not counted in the stats.
- An item card can apply its item to a stuff the player picks in a dialog ("Appliquer à…"), replacing what holds its place, with the forgemagic the search gave it (user requests).
- The search can be told items every stuff must wear, wherever they fit (user request); they come before the items kept from the current stuff.
- Side panels are resizable and characteristic fields are searchable by typing (user requests).
- In the builder (user requests): right-click and Delete remove an item, on its slot and on its card; Ctrl+Z undoes and Ctrl+Shift+Z redoes item changes; hovering an item shows its card with its stats.
- The search takes base characteristics into account (user request): it spends the characteristic points by the game's tiers (1, 2, 3 then 4 points per point; Vitalité 1 for 1, Sagesse 3 for 1) and counts each characteristic scrolled to 100, both by default.
- PA, PM and PO are capped at 12 / 6 / 6: gear may exceed the cap, the game keeps the cap (user rule).
- Prysmaradites only go in the first Dofus position, one per character; this is checked everywhere (user rule).
- The stuff's level is a ceiling in the search, never a floor (user rule, 2026-11-17): items above it are left out, and a lower-level item is worn when it serves the request better. There is no minimum item level.
- The characteristics to maximize are ranked (user request, 2026-11-17): the list's order is the priority, and each one counts ten times less than the one above. The factor of ten is an assumption, not a user figure.
- The search always fills every slot with the best combination, pets and mounts included (user requests); a slot stays empty only when filling it would make the stuff invalid.
- Switching between screens keeps each screen's state: filters, search results, scroll position (user request).
- Privacy (user rule, 2026-11-17): the app never asks for the player's account or any personal information, and everything is stored locally. The network is used only to download the game data and to update the app. No analytics, telemetry, ads or logins.
- Stuffs are shared without a server (user request, 2026-11-17): the desktop app copies the stuff's code, the web version a link that carries it; "Importer" in Mes équipements takes either one and opens the stuff in a new tab. The code holds the whole stuff (items, forgemagic, class, characteristics), nothing personal.
- A stuff belongs to a class (user request, 2026-11-17).
- The Stuff screen has a tab bar across its full width (user request, 2026-11-17): two tabs never close, "Mes équipements", which lists every stuff in a searchable table, and "Recherche de stuffs", which is no longer a screen of its own since it depends on a stuff; opening a stuff puts it in its own tab, and closing a tab keeps the stuff. The search targets one stuff; a result is opened in a new tab or applied to a stuff the player picks in a dialog (user request).
- Long lists come in batches rather than all at once (user request, 2026-11-17): 50 stuffs in the list, 30 in the "apply to" dialog with a search, 5 search results out of up to 20. Item icons in search results are twice their former size, and icons are imported at 128 px so they stay sharp (user decisions; the icon download is 166 MB instead of 64).
- The search can be told items no stuff may wear (user request, 2026-11-17), from its form or from a result's card. A forbidden item is left out even when the stuff wears it and "keep equipped" is on; an item both required and forbidden is worn.
- Two stuffs can be compared (user request, 2026-11-18): "Comparer avec…", on a stuff and on each search result, opens a dialog that says what going from the first stuff to the second gains and loses: characteristics, items place by place with their forgemagic, and what the weapon and the spells hit for. From a stuff, the other side opens on the stuff changed last; from a result, the comparison starts from the stuff the search targets; either side can then be any saved stuff or any result. Places of the same kind are interchangeable (the two rings, the six Dofus), so an item that only moved is not a difference, and the spells are those of the first stuff's class: both are assumptions, not user rules.
- A new stuff is created from a dialog that requires a name and a class, and deleting a stuff always asks for confirmation (user requests, 2026-11-17).
- Side panels show only when they have something to say (user request): the characteristics beside an open stuff, the item picker once a slot is chosen, nothing beside the list.
- The stuff page shows what the weapon and each spell of the class hit for, as dofusbook does (user request): direct damage and steals, normal and critical, on a target with no resistance. Poisons, triggered effects, heals and conditional bonuses are not computed.
- "Dommages finaux" is the main thing to maximize (user request, 2026-11-17: "je veux trouver le stuff qui tape le plus fort possible", without naming a spell or a weapon; final and critical damage are what matters). It reads the average hit of the class's spells in one element, each with its own critical chance, and the search picks the element itself unless the player sets it. Without a class, a reference spell stands in (28 to 32 base, 34 to 38 critical, 10% critical: an assumption).
- "Dommages finaux" can cover several elements (user request, 2026-11-18: players build bi-element and multi-element stuffs, with a main element or a balance to find). Two ways to count them, both kept at the user's request: a main element with a floor for each other one, as a percentage of the main one (100% everywhere is a balance), which gives the share asked for; or a weighted sum, which follows a rotation but lands on a share nobody chose. Measured on an Ecaflip, Water and Fire: a sum weighted 70/30 puts Fire at 82% of Water, 60/40 at 100%, and 50/50 makes Fire the main element. Elements are named by the player; the search does not pick a pair itself. Each element is read on every spell of the class, or on one spell or the weapon the player picks (user request, 2026-11-18: the class average surprised players, a Cra hits harder in Earth than in Water, so "Water at 70% of Earth" on the averages put more points in Chance than in Strength; naming Flèche du Jugement and Flèche d'Expiation gives the split the player expects).
- The list of what the search maximizes holds stats, weapons and spells alike, ranked together (user request, 2026-11-17: "je veux maximiser les dommages de tel sort ou de tel coup d'arc").
- The search accepts damage goals (user request): what a weapon or a spell hits for must reach a value (a condition) or be maximized (an entry of that list). It measures a normal hit, a critical hit, or the average final damage, which weighs both by the stuff's critical rate (user request, 2026-11-17: the search must find the best ratio in final damage, not follow a list of stats blindly).
- dofusbook (d-bk.net) is the reference for formulas (user rule, 2026-11-17). Checked on one of the user's stuffs rebuilt item for item: the 25 stats, 15 Ecaflip spells line by line, and the weapon's lines, total, critical rate and average all match. The weapon needs its mastery (+300 Power, on by default as on dofusbook) and its elemental forgemagic to match.
- Forgemagic is chosen with the rest of the stuff, on the stuff's score (user request): a rune that fills a missed condition comes before one that adds to what is maximized. A rune can land on a negative line, and a negative Critical line can be forged up to -1 (optional in the search). The -1 limit is read off the user's stuff, not confirmed as a game rule.
- The search cannot try every combination (about 40 items for each of 16 positions): it explores millions of stuffs with random restarts, crosses the best ones and re-chooses their forgemagic. The same request always gives the same answer; another random draw can still land a little higher or lower on stat-only searches.
- Not in scope yet: target resistances, poisons and triggered spell effects, heals, dofusbook's "critical mastery" option. User accounts are out of scope for good (privacy rule above).
- Versions start at 0.1.0 (user decision, 2026-11-17). Every `package.json` carries the same version; the release tag must match it.

## Brand Commitments

- "Modern design" requested; the stuff builder should stay visually similar to dofusbook's so players are not lost.
- A dark theme and a light theme, with no green in the surfaces (user decision, 2026-11-16). Green remains only where the game uses it: valid state, Agility and Air, MP.
- Name: Dofus Gear Explorer (user decision, 2026-11-17; was Dofus Database Explorer). The icon is the brand mark, four cells, on a dark rounded tile; no Ankama artwork.
- The app says everywhere it is shown that it is an unofficial tool, not affiliated with Ankama (the About and Legal notice menu, the README, the LICENSE, the native About panel). The legal notice names no publisher and points to GitHub issues for any request, Ankama's included. Free and non-commercial: no sales, ads or paid features.
- doduda and the dofusdude project are credited in the app and thanked in the README (user request).
- Game data and item artwork belong to Ankama.

## Evidence on Hand

- Real game data: 3,679 equipment items and 501 sets from release 3.7.13.17, with item icons, in `apps/api/data` after `pnpm data:sync`.
- No testimonials, user numbers or benchmarks exist; none may be invented.

## Product Principles

1. The game data is the authority: show what the game files say, and say which release it is.
2. A player's vocabulary first: labels, stat names and layouts follow the game and dofusbook conventions.
3. Numbers must be scannable: stats, ranges and totals are the content; everything else stays out of their way.
4. Never show a stuff as valid when a condition or constraint is not met.
5. The player's data stays theirs: no account, nothing leaves the computer, and every network request has a reason the player was told about.
