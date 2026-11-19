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
    for filename, textures in TEXTURES.items():
        environment = UnityPy.load(str(bundles / filename))
        required = set(textures.values())
        if filename == "uielements_assets_all.bundle":
            required.update(FONTS.values())
        for obj in environment.objects:
            if obj.type.name not in {"Texture2D", "Font"}:
                continue
            name = obj.peek_name()
            if obj.type.name == "Texture2D" and name in textures:
                stem = textures[name]
                obj.read().image.save(destination / f"{stem}.png")
            elif obj.type.name == "Font" and name in FONTS:
                stem = FONTS[name]
                (destination / f"{stem}.ttf").write_bytes(bytes(obj.read().m_FontData))
            else:
                continue
            required.discard(stem)
        if required:
            raise ValueError(f"{filename} is missing combat resources: {', '.join(sorted(required))}")
