"""Read combat interface artwork from the installed client's addressables."""

from pathlib import Path

import UnityPy


# Several bundles contain sprites with the same names at different sizes.
# In particular, the 208 × 64 end-turn texture matches the desktop fight HUD.
TEXTURES = {
    "figs_assets_2x.bundle": {
        "gaugeHeartBackground": "heart-background",
        "gaugeHeartFilledRed": "heart-fill",
        "paCounter": "ap",
        "pmCounter": "mp",
    },
    "uielements_assets_all.bundle": {"gaugeHeartBorder": "heart-border"},
    "uiassets_assets_.bundle": {
        "defaultBackgoundButton": "end-background",
        "defaultContentButton": "end-idle",
        "hoverContentButton": "end-hover",
        "pressedContentButton": "end-pressed",
        "defaultLightEffect": "end-light",
        "hoverLightEffect": "end-hover-light",
    },
    "uiassets_assets_2x.bundle": {
        "characteristic": "characteristic",
        "spells": "spells",
        "inventory": "inventory",
        "cosmetics": "cosmetics",
    },
    "uidarkstone_assets_all.bundle": {
        "background_timeline_blue_normal": "order-ally",
        "background_timeline_red_normal": "order-enemy",
    },
}

# Optional on older clients: the inventory is still usable without its ornamentation.
INVENTORY = {
    "figs_assets_.bundle": {
        "bgOrnamentTop": "inventory-top", "bgOrnamentBottom": "inventory-bottom",
    },
    "uiassets_assets_.bundle": {
        "ornamentBrakmar": "inventory-header", "headerHighlight": "inventory-highlight",
        **{f"socle_{breed}": f"inventory-pedestal-{breed}" for breed in [*range(1, 19), 20]},
    },
    "uidarkstone_assets_all.bundle": {
        "inventoryEmptySlot": "inventory-slot",
        **{f"icon_slot_{game}_inventory": f"slot-{slot}" for slot, game in (
            ("hat", "helmet"), ("cloak", "cape"), ("weapon", "weapon"), ("shield", "shield"), ("pet", "pet"),
            ("amulet", "collar"), ("ring", "ring"), ("belt", "belt"), ("boots", "shoe"), ("dofus", "dofus"))},
    },
}

FONTS = {
    "Lexend-Regular": "lexend-regular",
    "Lexend-Bold": "lexend-bold",
    "Rowdies-Regular": "rowdies-regular",
}


def addressables_directory(content: Path) -> Path:
    # Windows and Linux normally keep both Content and aa in StreamingAssets.
    # macOS puts the updateable Content next to the .app instead.
    roots = [content.parent / "aa"]
    for ancestor in [content.parent, content.parent.parent, content.parent.parent.parent]:
        roots.extend(ancestor.glob("*.app/Contents/Resources/Data/StreamingAssets/aa"))
        roots.extend(ancestor.glob("*_Data/StreamingAssets/aa"))
    for root in roots:
        for marker in root.glob("*/figs_assets_2x.bundle"):
            return marker.parent
    raise FileNotFoundError(f"Combat HUD bundles not found alongside {content}")


def export_hud(content: Path, output: Path) -> None:
    bundles = addressables_directory(content)
    destination = output / "hud"
    destination.mkdir(parents=True, exist_ok=True)
    for filename in sorted(TEXTURES.keys() | INVENTORY.keys()):
        textures = TEXTURES.get(filename, {})
        optional = INVENTORY.get(filename, {})
        bundle = bundles / filename
        if not textures and not bundle.is_file():
            continue
        environment = UnityPy.load(str(bundle))
        required = set(textures.values())
        if filename == "uielements_assets_all.bundle":
            required.update(FONTS.values())
        for obj in environment.objects:
            if obj.type.name not in {"Texture2D", "Font"}:
                continue
            name = obj.peek_name()
            if obj.type.name == "Texture2D" and (name in textures or name in optional):
                stem = textures.get(name) or optional[name]
                image = obj.read().image
                if stem.startswith("inventory-pedestal-"):
                    bounds = image.convert("RGBA").getchannel("A").getbbox()
                    if bounds:
                        image = image.crop(bounds)
                image.save(destination / f"{stem}.png")
            elif obj.type.name == "Font" and name in FONTS:
                stem = FONTS[name]
                (destination / f"{stem}.ttf").write_bytes(bytes(obj.read().m_FontData))
            else:
                continue
            required.discard(stem)
        if required:
            raise ValueError(f"{filename} is missing combat resources: {', '.join(sorted(required))}")
