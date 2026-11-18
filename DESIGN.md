---
name: Dofus Gear Explorer
description: The game's combat board as a working surface for reading items, assembling 16 slots and searching stuffs.
colors:
  ground: "oklch(0.955 0.006 258)"
  tile: "oklch(0.925 0.011 258)"
  panel: "oklch(0.992 0.002 258)"
  panel-2: "oklch(0.968 0.005 258)"
  line: "oklch(0.885 0.009 258)"
  line-strong: "oklch(0.72 0.018 260)"
  ink: "oklch(0.24 0.025 264)"
  ink-2: "oklch(0.42 0.025 262)"
  ink-3: "oklch(0.5 0.022 260)"
  cell-bg: "oklch(0.925 0.011 258)"
  slot-bg: "oklch(0.992 0.002 258)"
  accent: "oklch(0.5 0.17 258)"
  accent-ink: "oklch(0.99 0.005 258)"
  accent-soft: "oklch(0.93 0.04 256)"
  ok: "oklch(0.5 0.13 152)"
  ok-soft: "oklch(0.94 0.05 152)"
  bad: "oklch(0.53 0.2 27)"
  bad-soft: "oklch(0.945 0.035 27)"
  el-earth: "oklch(0.5 0.09 62)"
  el-fire: "oklch(0.56 0.19 38)"
  el-water: "oklch(0.54 0.13 240)"
  el-air: "oklch(0.54 0.14 150)"
  el-neutral: "oklch(0.52 0.012 250)"
  st-vitality: "oklch(0.55 0.19 12)"
  st-wisdom: "oklch(0.5 0.16 305)"
  st-ap: "oklch(0.5 0.17 258)"
  st-mp: "oklch(0.52 0.14 135)"
  st-range: "oklch(0.5 0.1 200)"
  st-critical: "oklch(0.56 0.15 55)"
  st-prospecting: "oklch(0.55 0.12 88)"
typography:
  headline:
    fontFamily: "Figtree Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 700
    lineHeight: 1.4
  title:
    fontFamily: "Figtree Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.15rem"
    fontWeight: 700
    lineHeight: 1.25
  section:
    fontFamily: "Figtree Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.93rem"
    fontWeight: 700
    lineHeight: 1.45
  body:
    fontFamily: "Figtree Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.45
    fontFeature: "\"tnum\" 1"
  data:
    fontFamily: "Figtree Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.93rem"
    fontWeight: 700
    lineHeight: 1.45
    fontFeature: "\"tnum\" 1"
  label:
    fontFamily: "Figtree Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.86rem"
    fontWeight: 600
    lineHeight: 1.45
  numeral:
    fontFamily: "Figtree Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.3rem"
    fontWeight: 800
    lineHeight: 1.2
    fontFeature: "\"tnum\" 1"
rounded:
  pip: "2px"
  control: "6px"
  slot: "8px"
spacing:
  chip-gap: "0.375rem"
  cell-gap: "0.5rem"
  control-gap: "0.75rem"
  panel: "1rem"
  section: "1.25rem"
  stage: "1.5rem"
components:
  button-default:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    height: "2rem"
    padding: "0 0.75rem"
  button-default-hover:
    backgroundColor: "{colors.panel-2}"
  button-solid:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.accent-ink}"
    rounded: "{rounded.control}"
    height: "2rem"
    padding: "0 0.75rem"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.ink-2}"
    rounded: "{rounded.control}"
  button-ghost-hover:
    backgroundColor: "{colors.panel-2}"
    textColor: "{colors.ink}"
  icon-button:
    backgroundColor: "transparent"
    textColor: "{colors.ink-2}"
    rounded: "{rounded.control}"
    size: "2rem"
  icon-button-hover:
    backgroundColor: "{colors.panel-2}"
    textColor: "{colors.ink}"
  chip:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink-2}"
    rounded: "{rounded.control}"
    height: "1.9rem"
    padding: "0 0.6rem"
  chip-pressed:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.accent-ink}"
  text-input:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    height: "2.3rem"
    padding: "0 0.7rem"
  num-input:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    height: "2rem"
    width: "4.4rem"
    padding: "0 0.5rem"
  num-input-invalid:
    textColor: "{colors.bad}"
  nav-link:
    textColor: "{colors.ink-2}"
    padding: "0 0.85rem"
  nav-link-current:
    textColor: "{colors.ink}"
  item-cell:
    backgroundColor: "{colors.cell-bg}"
    textColor: "{colors.ink-3}"
    rounded: "{rounded.control}"
    size: "2.75rem"
  slot:
    backgroundColor: "{colors.slot-bg}"
    textColor: "{colors.ink-3}"
    rounded: "{rounded.slot}"
    size: "4.5rem"
  slot-selected:
    backgroundColor: "{colors.accent-soft}"
  slot-invalid:
    backgroundColor: "{colors.bad-soft}"
  table-header:
    backgroundColor: "{colors.tile}"
    textColor: "{colors.ink-2}"
    height: "2.4rem"
    padding: "0 0.45rem"
  table-row:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    height: "48px"
    padding: "0 0.45rem"
  table-row-hover:
    backgroundColor: "{colors.panel-2}"
  table-row-selected:
    backgroundColor: "{colors.accent-soft}"
  vital:
    backgroundColor: "{colors.panel-2}"
    textColor: "{colors.ink-2}"
    typography: "{typography.numeral}"
    rounded: "{rounded.slot}"
    padding: "0.55rem 0.7rem"
  goal:
    backgroundColor: "{colors.ok-soft}"
    textColor: "{colors.ink}"
    height: "1.7rem"
    padding: "0 0.5rem"
  goal-unmet:
    backgroundColor: "{colors.bad-soft}"
  banner:
    backgroundColor: "{colors.accent-soft}"
    textColor: "{colors.ink}"
    padding: "0.6rem 1rem"
  banner-error:
    backgroundColor: "{colors.bad-soft}"
---

# Design System: Dofus Gear Explorer

## Overview

**Creative North Star: "The Combat Board"**

*Naming note: the North Star and the descriptive colour names in this file (Range Blue, Movement Green, Impact Red, Board Slate and the rest) were derived from the build's direction contract. The user has not confirmed them yet; the token names in `styles.css` are the stable reference.*

The interface borrows the surface a Dofus player already stares at in a fight: a tinted ground laid out in a two-tone checker, square cells, and the three zone colours the game uses to say what a cell means. Blue is where the player's hand is (the game's spell range), green is what is valid (movement), red is what fails (impact). Characteristics are read in the colour of their element. Nothing else on the screen carries hue, so every colour a player sees is information they already know how to read.

It is a working tool, dense on purpose. The root size is small (14px), rows are fixed and compact, numbers are tabular and bold, and labels step back to a softer ink so that values are what the eye lands on. The two screens, Items and Stuff, share one frame: a top strip with the two destinations, the data release and the app controls, then columns that fill the window and scroll on their own. The Stuff screen holds the stuff list, the stuff search and every open stuff as tabs of one full-width bar. There is no page chrome, no hero, no marketing voice.

It refuses two looks by construction: the dark neon gamer dashboard (no glow, no gradients on controls, the dark theme is the same slate board at night, not black) and the white SaaS admin table (the ground is tinted, state is carried by game colours, item cells and slots are the recurring shape instead of cards).

Tokens live in one place, `packages/app/src/styles.css`: the light theme on `:root`, the dark theme on `:root[data-theme="dark"]`. That file is normative. The frontmatter above is the light theme as it stood when this was written; if a value there and the stylesheet disagree, the stylesheet is right and the role described here still holds.

**Key Characteristics:**

- A slate board ground in both themes, with no green in the neutrals (user decision): plain under the rails that hold controls, in a two-tone checker wherever pieces are placed or nothing has been picked or asked yet.
- Square cells for every item image and every equipment slot, in three sizes of the same shape.
- Three zone colours used only as state: blue selected, green valid, red failing.
- Twelve stat tones, resolved from the stat itself, shown as a cell-shaped pip or as the colour of the number.
- One typeface, four weights, tabular numerals everywhere.
- Flat panels separated by one-pixel lines; shadow only under things that float.
- Light and dark themes with the same roles; both are first-class.

## Colors

A quiet blue-grey slate ground and slate ink, with hue reserved for two jobs: zone colours for state, stat tones for characteristics.

### Primary

- **Range Blue** (`accent`, with `accent-ink` on top and `accent-soft` as its wash): the mark of selection and of the action the player is about to commit. It fills the one solid button a panel has, a pressed slot chip, the current destination's underline, the active sort header, the progress bar, the keyboard focus outline, and the small dot that says a data update is waiting (its own `notice` tone, so no stat colour is borrowed for it). Its wash is the fill of the selected table row, the hovered or selected slot, the equipped row in the picker and the data-update banner.

### Secondary

- **Movement Green** (`ok`, wash `ok-soft`): valid. The "stuff valid" status mark, the tick on a met condition, and the wash behind a met goal in solver results.
- **Impact Red** (`bad`, wash `bad-soft`): failing. An invalid slot, an unmet goal, the problems list under the character panel, the failed-update banner, an over-spent points message, an invalid number field, error text. It is also the colour of a negative value in a stat sheet, a table cell or an effect line.

### Tertiary

Stat tones. Each characteristic resolves to one tone through `toneOf` in `packages/app/src/lib/stats.ts`; an element's damage and resistance lines inherit the element's tone by key prefix, and anything unmatched is `plain`: it claims no colour and its pip is drawn hollow.

- **Earth Brown** (`el-earth`): Strength and every earth damage or resistance.
- **Fire Orange-Red** (`el-fire`): Intelligence and every fire line.
- **Water Blue** (`el-water`): Chance and every water line.
- **Air Green** (`el-air`): Agility and every air line.
- **Neutral Grey** (`el-neutral`): neutral damage and resistance.
- **Vitality Rose** (`st-vitality`): Vitality and hit points.
- **Wisdom Violet** (`st-wisdom`): Wisdom.
- **AP Blue** (`st-ap`): action points.
- **MP Green** (`st-mp`): movement points.
- **Range Teal** (`st-range`): range.
- **Critical Amber** (`st-critical`): critical hit.
- **Prospecting Gold** (`st-prospecting`): prospecting.

### Neutral

- **Board Slate** (`ground`) and **Checker Slate** (`tile`): the board. `ground` is the page and the fill of the filter rail; `tile` is the second tone of the checker, the table header, and the fill of loading skeletons.
- **Sheet** (`panel`) and **Sheet Shade** (`panel-2`): the surfaces data sits on. `panel` is the default; `panel-2` is one step down, used for hover rows, vital tiles, the results header and the builder's idle picker panel.
- **Cell Ground** (`cell-bg`) and **Slot Face** (`slot-bg`): what item art sits on, in an item cell and in an equipment slot. In the light theme they are the checker tone and the sheet; in the dark theme both are lifted well above the panels, because item art is often near black and must stay readable.
- **Cell Line** (`line`) and **Cell Edge** (`line-strong`): `line` divides panels and rows; `line-strong` outlines things the player can press or type into (inputs, chips, slots, default buttons) and underlines the table header.
- **Slate Ink** (`ink`, `ink-2`, `ink-3`): `ink` for values and names, `ink-2` for labels and supporting sentences, `ink-3` for the quietest text (item type under a name, a zero value, placeholders) and for empty slot glyphs.

In the dark theme the roles are identical and the board stays slate: the neutrals keep the same blue-grey hue, with `tile` raised far enough above `ground` for the checker to read. The zone colours and tones are lightened so they read as text, and the soft washes become dark tints of the same hue. `accent-ink` flips to a dark ink so the solid button stays legible.

### Named Rules

**The Zone Colour Rule.** Blue, green and red are states, never decoration. Blue says "this is selected, or this is the move"; green says "this passes"; red says "this fails or is negative". A surface that is none of those stays neutral.

**The Soft-Fill Rule.** State on a surface is the soft wash as fill, with the strong colour kept for the border, the icon or a two-pixel ring; text on a wash stays ink. The strong colour is used as a fill only where blue means "pressed" or "do it" (solid button, pressed chip).

**The Element Rule.** A stat's colour is decided by the stat, in one function. No screen assigns a stat colour locally, and a tone is applied only to a pip or to the number itself, never to a panel, a border or a button. A mark that is not a stat (the update dot) gets its own tone name instead of borrowing a stat's.

**The State Never Rides On Colour Alone Rule.** Every zone colour arrives with a second signal: a tick, cross or warning icon, a status word, a ring. A stuff is never shown valid by default; the mark appears only once the evaluation says so.

## Typography

**Display Font:** none; the system has no display face.
**Body Font:** Figtree Variable, self-hosted (with ui-sans-serif, system-ui, sans-serif)
**Label/Mono Font:** the same family; alignment of figures comes from tabular numerals, not from a mono face.

**Character:** one friendly geometric sans doing all the work, kept small and set heavy where it counts. Hierarchy comes from weight and ink level before it comes from size.

### Hierarchy

The root is 14px, so every rem below is a little smaller than it looks.

- **Headline** (700, 1.25rem): the one title a panel may carry, used for the search form heading and the setup and offline stages.
- **Title** (700, 1rem to 1.15rem, tight leading): the name of the thing in a detail panel (item name in the sheet, picker heading, results count, empty-state heading).
- **Section** (700, 0.93rem): group headings inside a panel (stat categories, "Effects", "Conditions", "Set"). Sentence case, ink, no tracking.
- **Body** (400, 1rem, 1.45): sentences, effect lines, row text. Supporting sentences use `ink-2` and are capped near 60 to 65 characters where they run long.
- **Data** (700, 0.93rem in the table, 1rem in sheets): values. Right-aligned in columns, tabular.
- **Label** (600, 0.86rem): field labels, the small label on a vital tile, and at regular weight the second line under an item name. Table headers use the same size at 700.
- **Numeral** (800, 1.3rem, 1.2): the four vital figures of a stuff (HP, AP, MP, range) and nothing else. The result rank uses the same weight one step smaller, in quiet ink.

Controls from the component library sit at 0.875rem with the same family; buttons and chips are weight 600.

### Named Rules

**The Numbers First Rule.** In any label-and-value pair the value is bold ink and the label is `ink-2`. A zero drops to weight 500 and `ink-3` so non-zero values stand out in a long sheet.

**The Tabular Rule.** Tabular numerals are on for the whole document. Numeric columns are right-aligned; names are left-aligned and truncate with an ellipsis rather than wrap.

**The Four Weights Rule.** 400 for prose, 600 for labels and controls, 700 for values and titles, 800 for vitals. No italics, no tracked uppercase labels, no letter-spacing other than a slight tightening on the product name.

## Layout

The app is a fixed frame the height of the window: a top strip (3.5rem) holding the brand mark, the two destinations, then on the right the data release, the language switch, the theme toggle, the app update button when a version waits (desktop only) and the About menu. An optional banner sits under it for data updates. Everything below fills the remaining height, and each column scrolls on its own; the page itself does not scroll on desktop.

Each screen is columns divided by one-pixel lines, in the same order of roles: what you set on one side, the work in the middle, the detail on the right.

- **Items:** the filter rail on the ground (21rem), the item table (fluid), the item sheet (24rem). With nothing selected, the third column is the checker with a small note card.
- **Stuff:** a tab bar across the full width, then the content of the selected tab. Side columns appear only when they have something to show.
  - **Mes équipements** (fixed tab): the searchable table of every stuff, full width, 50 rows at a time.
  - **Recherche de stuffs** (fixed tab): the conditions form (26rem), the results (fluid), 5 at a time.
  - **One tab per open stuff**: the stat sheet (20rem), the board with the 16 slots around the character panel (fluid), and the item picker (24rem) once a slot is chosen.

Responsive behaviour is structural, in three steps (Tailwind's `lg` at 64rem and `xl` at 80rem, plus 40rem for the board and the top strip):

- Below `xl`, the third column gives way. On Items, the sheet becomes a fixed panel over the right edge with the pop shadow. On the builder, the grid takes explicit rows: the board on top, the stat sheet in a shorter row under it, the picker down the full height on the right.
- Below `lg`, the page scrolls as one. On Items, the filter rail collapses behind a "Filters" toggle in the table's toolbar, and the name search moves into that toolbar so it is always in reach. On the builder, the order is board then stats, and the picker becomes a fixed bottom sheet (about three quarters of the height, pop shadow, its own close button) that appears when a slot is selected. On the search, the results scroll into view and take focus when a run finishes. The data release leaves the top strip below `md`; the product name is dropped below `sm` and only the mark remains.
- Below 40rem, the top strip wraps and the two destinations take their own full-width row; the board's slot columns become wrapping rows, the slot cell shrinks one step, and the character panel moves under the slots; the table's name column narrows.

Spacing follows a 0.25rem step. Panel padding is 1rem (1.25rem for the search form and result rows, 1.5rem for a stage card); groups inside a panel are separated by 1.25rem; controls in a row by 0.5rem; chips and cells in a wrap by 0.25rem to 0.375rem; slots on the board by 0.5rem. Table rows are a fixed 48px because the list is virtualised. The table scrolls sideways inside its column, and the name column stays pinned to the left edge while stat columns pass under it.

### Named Rules

**The Ground Rule.** The ground is for rails, stages and empty states; data sits on `panel`. Plain `ground` is the filter rail. The checker is where pieces are placed or where nothing is there yet: the builder's board, the item-sheet column before an item is picked, the setup and offline stages, the first load, the search tab before a run. Tables, lists and sheets sit on `panel`, edge to edge, with no ground showing between their rows.

**The Why-It-Matches Rule.** A stat the player filters on always gets a column in the table, so the reason a row is there is visible.

## Elevation & Depth

Flat. Panels are told apart by a one-pixel `line` and by small tonal steps between `ground`, `panel` and `panel-2`, not by shadow. The board's depth is its checker.

### Shadow Vocabulary

- **Pop** (`--shadow-pop`, a tight contact shadow plus a wide soft one, both in ink; heavier and black in the dark theme): the only shadow. It belongs to things that float above the frame: popovers, menus, select lists, toasts, the item sheet when it overlays the table on narrower windows, and the builder's picker when it is a bottom sheet.

The two-pixel ring on a selected slot is drawn with `box-shadow`, but it is a state outline, not elevation.

### Named Rules

**The Flat Board Rule.** Nothing resting on the frame casts a shadow. If it does not float over other content, it gets a line, not a shadow.

## Shapes

The cell is the shape of the system: a square with a small radius and a one-pixel edge. It appears at three scales that keep the same proportions, a pip, an item cell, a slot.

- **Pip corner (2px):** stat pips (filled in the stat's tone, or a hollow one-pixel outline when the stat has no tone), the four squares of the brand mark, the set marker in a slot corner.
- **Control corner (6px):** buttons, inputs, chips, icon buttons, item cells, the library's base radius.
- **Slot corner (8px):** equipment slots, vital tiles, popups, and the panels that sit on the board (character panel, problems list, stage and note cards).

Small badges inside a cell (the level tag, a goal mark, a skeleton block) sit between the pip and control corners. Borders are always one pixel. Nothing is clipped into circles; item art is shown whole, contained inside its cell with a margin. The switch is the one control that keeps the library's rounded track.

The checker is a conic gradient of `tile` over `ground` on a 4rem repeat, so each board square is 2rem.

Icons come in two kinds. Interface actions use Lucide outline icons at 1rem. The ten equipment slots have their own authored glyphs in `ui/SlotGlyph.tsx`: 24-unit outlines at a 1.6 stroke with round caps and joins, drawn in the current text colour. They mark an empty slot, label the slot filter chips, and stand in when an item image fails to load.

### Named Rules

**The Square Cell Rule.** Anything that is an item, a slot or a stat marker is a square cell. Item art never appears in a circle, a bare image or a card.

**The Pip Rule.** A characteristic is marked by one pip in its tone, never by a pictogram. The pip sits before the label; the label still names the stat. A stat with no tone keeps a hollow pip, so the column stays aligned without inventing a colour.

## Components

Controls are quiet and exact: one-pixel edges, small corners, colour only when something is selected or wrong. Transitions are short (160ms on controls, 220ms on the destination underline and slot press) on one ease-out curve; table rows change state without a transition, and all motion collapses under reduced motion.

### Buttons

Built on ui-sdk's button, themed through the `--ui-*` variables.

- **Shape:** control corner (6px), 2rem tall, weight 600, never wraps.
- **Default:** `panel` fill, `line-strong` edge, ink text; hover steps the fill to `panel-2`. This is the everyday button (add a condition, new stuff, open in the builder, unequip).
- **Solid:** `accent` fill with `accent-ink` text; hover mixes a little of the panel back in. One per panel at most: equip, run the search, update the data. The search's run button is taller (2.5rem) and spans the form.
- **Ghost:** transparent with `ink-2` text until hovered; for resets and for the data-release trigger in the top strip.
- **Icon button:** a 2rem square, transparent, `ink-2` icon, `panel-2` on hover. Always carries an accessible label, and a title where the icon alone could be misread.
- **Disabled:** half opacity, not-allowed cursor.
- **Focus (everything focusable):** a two-pixel `accent` outline offset by two pixels. Text and number inputs pull the outline in to the edge and turn their border `accent`; table rows inset it.

### Chips

- **Slot filter chip:** 1.9rem tall, `panel` fill, `line-strong` edge, `ink-2` text at weight 600, led by the slot's glyph. Hover darkens the fill; pressed turns it solid `accent`.
- **Chosen-stat chip** (stats to maximise): the same height and edge, a pip in the stat's tone, the stat name, and a small remove button inside.

### Item cell

The square that holds item art. `cell-bg` fill, `line` edge, control corner; the image is contained at about 86% of the cell. Sizes in use: 2.5rem in table rows and set lists, 2.75rem in picker rows and result strips, 4rem in the item sheet header. When the image fails, the slot's glyph takes its place in quiet ink. The image itself is decorative next to a written name; where cells stand alone (the strip of a search result) each one is a labelled image carrying the item's name.

### Equipment slot (signature)

The builder's cell, and the largest: 4.5rem (3.6rem below 40rem), `slot-bg` fill, `line-strong` edge, slot corner.

- **Empty:** the slot's glyph, in `ink-3`.
- **Filled:** the item art, which scales and fades in when it lands; the item level in a small tag at the bottom right; and, when the piece belongs to a set with at least two pieces equipped, a set marker (a pip-sized square) at the top left.
- **Hover:** `accent` edge, `accent-soft` fill. **Pressed:** shrinks very slightly. **Selected:** the same, plus a two-pixel `accent` ring; the picker (right column, or bottom sheet on small screens) now lists items for that slot.
- **Invalid:** `bad` edge and `bad-soft` fill when an equip condition or constraint fails for that piece; the ring turns `bad` when it is also selected.

The 16 slots are arranged as on dofusbook: five down the left (hat, cloak, amulet, two rings), five down the right (weapon, shield, belt, boots, pet), six Dofus and trophies in a row underneath, the character panel in the middle.

Set markers use a short cycle of six hues so two sets on the same stuff can be told apart, and the same hue marks the set in the bonus list. The hues are chosen to stay clear of the blue, green and red that mean selected, valid and failing, so a marker never reads as a state. The cycle is defined beside the builder component, not in the token file, at one lightness for both themes; it is the one colour series outside the tokens, and new work should extend it there or move it into the tokens rather than start a second inline series.

### Pips, sheet rows and effect lines

- **Pip:** a 0.6rem square, pip corner, filled with the stat's tone. A `plain` pip (a stat with no element or tone of its own) is hollow: a one-pixel `line-strong` outline and no fill. The `notice` pip is the same square in `accent`, used once, beside the data release when an update is waiting.
- **Sheet row:** pip, label in `ink-2`, value in bold ink pushed to the right; 1.75rem tall. Zero values go quiet, negative values go `bad`. Rows are grouped under section headings by the game's own stat categories.
- **Effect line:** pip, then the game's own text for the effect, baseline-aligned. Weapon damage lines lead and are semi-bold, with the pip in the damage element's tone. A negative line is `bad`. In the picker, effect lines flow in two columns.
- **Stat value in the table:** no pip; the number itself takes the stat's tone at weight 700, with the roll range in a tooltip. A negative maximum is `bad`.

### Vital tiles

Four tiles at the top of the stat sheet (HP, AP, MP, range): `panel-2` fill, `line` edge, slot corner, the label on the left and the figure on the right in the numeral style, coloured in the stat's tone.

### Inputs / Fields

- **Text input:** 2.3rem tall, `panel` fill, `line-strong` edge, control corner, placeholder in `ink-3`. Labels sit above at the label size.
- **Number field:** a compact 2rem by 4.4rem box, right-aligned, weight 600, with no steppers; the content is selected on focus so a new figure replaces the old one. Invalid turns the border and the text `bad`.
- **Select:** the library's trigger at control height with a chevron; long values truncate. Stat lists are grouped by category, each option led by its pip.
- **Condition row:** stat select, operator select (≥ or ≤), number field, remove button, on one line. Used identically for item filters and for solver conditions.
- **Switch, segmented control:** library defaults on the system's tokens; the checked switch and the selected segment follow the library's own treatment.

### Data table

- **Header:** sticky, on `tile`, `line-strong` underline, bold text at the label size in `ink-2`; each header is a sort button with a faint up-down arrow that turns `accent` and solid when that column sorts.
- **Rows:** 48px, `line` underline, on `panel`; hover `panel-2`, selected `accent-soft`. One row-background variable drives every cell of the row, so the pinned name column always matches the rest. The first column holds an item cell, the name in semi-bold and the type and set in the label size beneath; it is pinned to the left with a `line` on its right edge. Every other column is right-aligned.
- **Loading:** rows of skeleton blocks in `tile` that pulse; **empty:** a short title and one sentence, top-left of the table area; **error:** one sentence in `bad`.

### Status and goal marks

- **Status:** icon plus word, weight 700, in `ok` or `bad` (the stuff's validity in the builder toolbar).
- **Goal:** a 1.7rem tag for each solver condition on a result: tick or cross, the stat label in `ink-2`, the reached value in bold, on `ok-soft` when met and `bad-soft` when not. Stats that were not conditions follow as pip, label, value with no tag.

### Navigation

Two text links in the top strip (Items, Stuff), weight 600, `ink-2`, turning `ink` on hover and when current. The current destination gets a three-pixel `accent` bar that grows from the centre along the strip's bottom edge. Below 40rem the links take a full-width row of their own under the mark and the controls, each sharing the width equally. The brand mark is four pips in a two-by-two block: accent, ok, bad, and a neutral edge.

### Stuff tabs

A bar across the Stuff screen, on `panel-2` with a `line` under it. Tabs are 2.25rem high with a 6px top corner and no bottom edge; the selected one takes `panel`, the `line` edge on three sides and sits one pixel over the bar's line so it joins the content below. Labels are weight 600 in `ink-2`, `ink` when selected or hovered. Two tabs come first and never close: Mes équipements and Recherche de stuffs, each with an icon. Then one tab per open stuff: its name, its class in `ink-3`, and a close button; a "+" at the end creates a stuff. With more tabs than fit, the bar scrolls sideways without a scrollbar and keeps the selected tab in view.

The Mes équipements toolbar ends with two buttons: Importer (outlined, paste icon) and Nouveau stuff (solid). Importer opens a 32rem dialog with one textarea for a code or a link; under it a single status line says, as the player pastes, either what will be imported (name · class · level · item count in `ink-2`) or that the text is not a stuff (in `bad`), and the Import button stays disabled until the text reads as a stuff.

### About menu and dialogs

The last control of the top strip is an icon button (info icon) named "À propos et mentions légales". Its menu opens with the product name and the version in `ink-3`, then items with icons: About and Legal notice, and on desktop "Rechercher une mise à jour", which stays pending while the app looks and answers with a toast. About and Legal notice each open a dialog (34rem wide at most, height capped to the window):

- **About:** the product name, one sentence on what it does, a two-column list of facts (app version, game data version, licence) with values in bold tabular figures, the doduda credit and the unofficial line in `ink-2`, then links in `accent`, weight 600: the source code, led by the GitHub mark, and the third-party licences, followed by a small external-link icon.
- **Legal notice:** titled sections (unofficial tool, trademarks and game content, your data, calculations, contact and removal, licence), each a bold title over `ink-2` text; the body scrolls while the title and the Close button stay.

### Compare dialog

"Comparer avec…" is a default button led by a two-arrow icon, in a stuff's toolbar and on each search result. Its dialog is up to 62rem wide and 50rem high and keeps that size whatever it shows: the title and a close icon button, then the two stuffs as searchable fields (Stuff de départ, Stuff d'arrivée) with a swap icon button between them. Under each field one line in `ink-3` gives the class, the level and the item count, with the red status mark when the stuff is invalid. Below 40rem the fields stack and the swap button stands beside them.

The rest scrolls. Four tiles in the vital-tile shape come first (HP, AP, MP, range): the stat's pip and label, the change in the numeral style, and "before → after" in `ink-2`. Then three tabs underlined in accent like the slot panel's, each with its count of changes in `ink-3`:

- **Caractéristiques:** gains and losses in two columns (one below 40rem), each row a pip, the label, the two values and the change; the stats that do not move wait under a disclosure.
- **Objets:** one row per place that changes: the slot's glyph and name, then the item of each side in a 2.5rem cell with an arrow between them, and "Emplacement vide" in `ink-3` where a side wears nothing. The same item forged differently shows once, with the lines that differ and their origin tag. Sets whose count changes and the character's own differences (level, points, scrolls) follow as small tables.
- **Dommages:** one table for the weapon, the class's spells and the common ones, with a segmented control for the hit that is read (average, normal, critical) and the change in percent in `ink-3`.

A change is written with its sign and takes `ok` when the value goes up and `bad` when it goes down, on the number only; an unchanged value is "=" in `ink-3`. This is the one place where green and red mean more and less rather than valid and failing, so the sign and the Gains and Pertes headings, each with an arrow, say it without the colour.

### Update notice

When a new version of the desktop app is ready, a toast stays until dismissed: title with the version, one sentence, one action ("Redémarrer" when it installs on restart, "Télécharger" when it can only point to the download page). It uses the regular toast styling.

The toast can be dismissed, so the top strip keeps the offer: an icon button appears just before the About button while a version waits, and is absent the rest of the time and on the web. Its tooltip is its name and gives the version. While the version downloads it is a spinning loader in `ink-2` that ignores presses (`aria-disabled`); once it is ready, an arrow-up-in-circle in `accent` on the `accent-soft` wash, and one press installs and restarts; when the app can't replace itself, a download icon in the same colours that opens the release page. The wash follows the Soft-Fill Rule, and the state is carried by the icon, not by the colour.

### App icon

The desktop icon is the brand mark at scale: the four cells (accent blue, ok green, bad red, neutral) with rounded corners, on a dark slate rounded tile with a faint top-to-bottom gradient and a hairline edge. Source in `apps/desktop/build/icon.svg`, rendered to a 1024px PNG. No Ankama artwork, ever.

### Banner, stage and popups

- **Banner:** a full-width strip under the top bar. `accent-soft` when new game data is available or importing (with a progress bar and the step name; the bar's fill is `accent` and moves to its value without a width animation), `bad-soft` when an update failed. Message on the left, one button on the right.
- **Stage:** a single `panel` card (slot corner, `line` edge, 1.5rem padding, at most about 28rem wide) centred on the checker, for first import, backend offline, and the search tab's idle state. It holds a title, one or two sentences, and one action or a progress bar. The smallest form is the note card in the item-sheet column before an item is picked: one centred sentence in `ink-2`, same fill, edge and corner.
- **Popups** (popover, select list, menu, toast): `panel` fill, `line` edge, slot corner, the pop shadow; they fade and scale in slightly from their anchor.
- **Result row:** rank in heavy quiet ink, the goal marks, an open button, then a strip of item cells in slot order and the sets in one `ink-2` line; rows are separated by a `line`, not boxed.

### Tags (forgemagic origin)

A tag is a small outlined label, 4px radius, bold, in ink: it names where a line comes from (Over, Exo, Transcendance, Dracorune) or marks a game version (MàJ 3.8). Tags carry no hue: blue, green and red stay reserved for state, and the stat keeps its pip. A forged item on the board shows one small ink sparkle in its top-right corner, on a panel-coloured chip.

### Resizable columns

From lg up the side panels are columns the player drags wider or narrower. The handle is the 1px line between columns; on hover, focus or drag it turns accent blue and doubles in width. Widths are remembered per screen. Below lg there are no handles: panels stack, and the slot panel becomes a bottom sheet.

### Searchable select

Any field that picks a characteristic or a rune is a text field with a chevron: typing narrows the list, accents ignored ("res" finds Résistance). Options keep the stat's pip. A field used to add something (an exo, a stat to maximize) empties itself after each pick.

### Slot panel tabs and context menu

The right panel of the builder has two tabs, Objets and Forgemagie, underlined in accent when selected. A right-click on a worn item opens a short menu: change item, forgemagic, remove. Empty slots keep the browser's own menu.

### Item hover card and worn-item cards

Resting the pointer on an item (a board slot, a search result cell, a set piece) opens its card: icon, name, type and level, every line as it counts with its origin tag, then condition and passive. The card is a panel with the strong line and the one shadow, 17rem wide. Under the board, the same content is laid out as a grid of cards, one per worn item; a card is selected with the accent border like a slot, and takes the same right-click menu and Delete key.

## Do's and Don'ts

### Do:

- **Do** take every colour from the tokens in `styles.css` and define both themes when adding one.
- **Do** resolve a stat's colour through `toneOf` and show it as a pip or as the number's colour.
- **Do** pair every zone colour with an icon, a word or a ring, and keep text on a wash in ink.
- **Do** put item art in a square cell at one of the existing sizes, with the slot glyph as its fallback.
- **Do** keep values bold and right-aligned, labels in `ink-2`, and zero values quiet.
- **Do** keep one solid blue button per panel; everything else is default or ghost.
- **Do** separate panels with one-pixel lines and let each column scroll on its own.
- **Do** keep the ground for rails, stages and empty states: plain under a rail of controls, checker behind slots and wherever nothing has been picked or asked yet.
- **Do** put item art on `cell-bg` or `slot-bg`, never straight on a panel, so dark art stays readable in the dark theme.
- **Do** give a mark that is not a stat its own tone name, and keep set markers outside the blue, green and red state hues.

### Don't:

- **Don't** use `accent`, `ok` or `bad` to decorate; if a surface is not selected, valid or failing, it stays neutral.
- **Don't** apply a stat tone to a panel, border or button, and don't give a stat a colour outside the tone map.
- **Don't** replace pips with stat pictograms, or item cells with circles, bare images or cards.
- **Don't** add shadows to panels, rows or cells; only floating layers cast one.
- **Don't** tint the neutrals green: the user asked for dark and light themes without green. Green stays for what the game makes green: valid, Agility and Air, MP.
- **Don't** add glow, gradient fills on buttons, panels or text, or a black neon dark theme; the dark ground is the same slate, at night.
- **Don't** lay data tables, lists or sheets on the ground or the checker, or box result rows into cards.
- **Don't** show a stuff as valid before its evaluation says so, or hide a failing condition behind colour alone.
- **Don't** introduce a second typeface, tracked uppercase labels, or weights outside the four in use (a quiet zero at 500 is the one exception).
