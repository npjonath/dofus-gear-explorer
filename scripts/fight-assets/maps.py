"""Bake the two Kanojedos and their combat grids from DOFUS 3's local map bundles."""

import json
import re
import sys
from itertools import groupby
from pathlib import Path

import UnityPy
from PIL import Image, ImageChops


VILLAGE = 99222029
ASTRUB = 192419850
HALF_W = 43
HALF_H = 21.5
LAYERS = ("background", "middleground", "foreground")


def _client_unity_version(content):
    # The UnityFS header can say 0.0.0 even on a complete install. Its player
    # data has the real Unity build in the first bytes of globalgamemanagers.
    root = content.parent.parent
    player_data = (
        root, root / "Dofus_Data",
        root / "Dofus.app" / "Contents" / "Resources" / "Data",
    )
    for folder in player_data:
        file = folder / "globalgamemanagers"
        if file.is_file():
            with file.open("rb") as stream:
                header = stream.read(160)
            version = re.search(rb"\b(\d{4}\.\d+\.\d+[abfp]\d+)\x00", header)
            if version:
                return version.group(1).decode("ascii")
    return "6000.3.16f1"


def _asset_name(address):
    # The catalog's separator and case belong to the client that built it.
    return address.replace("\\", "/").rsplit("/", 1)[-1].casefold()


def _bundle_path(root, *parts):
    wanted = root.joinpath(*parts)
    if wanted.is_file():
        return wanted
    current = root
    for part in parts:
        if not current.is_dir():
            return wanted
        exact = current / part
        if exact.exists():
            current = exact
        else:
            current = next((p for p in current.iterdir() if p.name.casefold() == part.casefold()), exact)
    return current


def _read_map(content, map_id):
    world = map_id >> 18
    bundle = _bundle_path(content, "Map", "Data", f"mapdata_assets_world_{world}.bundle")
    if not bundle.is_file():
        return None
    package = UnityPy.load(str(bundle))
    filename = f"map_{map_id}.asset"
    for key, asset in package.container.items():
        if _asset_name(key) == filename:
            return asset.deref_parse_as_dict()
    return None


def _render_elements(data):
    references = {ref["rid"]: ref.get("data", {}) for ref in data["references"]["RefIds"]}
    for layer in LAYERS:
        materials = data[f"{layer}MaterialData"]["shaderData"]
        for group in data[f"{layer}MapElements"]:
            shader = references[materials[group["materialIndex"]]["rid"]]
            parameters = [references[p["rid"]] for p in shader["shaderParameters"]]
            blend = next((p for p in parameters if "sourceFactor" in p), None)
            for ref in group["mapElements"]:
                yield references[ref["rid"]], blend


def _read_texture(asset, size):
    obj = asset.deref_parse_as_object()
    if asset.deref().type.name == "Texture2D":
        tex, units = obj.image, size
    else:
        # The map places full textures by their centre. Sprite.image cuts away
        # transparent padding and shifts adjacent floor tiles.
        tex = obj.m_RD.texture.deref_parse_as_object().image
        units = obj.m_PixelsToUnits
        sprite_size = (round(obj.m_Rect.width), round(obj.m_Rect.height))
        if sprite_size != tex.size:
            trimmed = obj.image
            offset = obj.m_RD.textureRectOffset
            tex = Image.new("RGBA", sprite_size)
            tex.paste(trimmed, (round(offset.x), tex.height - trimmed.height - round(offset.y)))
    if units != 1:
        tex = tex.resize((max(1, round(tex.width / units)), max(1, round(tex.height / units))),
                         Image.Resampling.LANCZOS)
    return tex.convert("RGBA")


def _read_textures(content, gfx_ids):
    textures = {}
    # Clients can have different texture resolutions installed. Texture coordinates
    # in the map itself always use the 1x grid.
    for bucket, ids in groupby(sorted(gfx_ids), key=lambda gid: gid // 10000 * 10000):
        missing = set(ids)
        for size in (1, 2, 4):
            file = _bundle_path(content, "Map", "Textures", f"{size}x", f"mapgfx_{size}x_{bucket}_assets_all.bundle")
            if not file.is_file():
                continue
            package = UnityPy.load(str(file))
            for key, asset in package.container.items():
                stem, _, suffix = _asset_name(key).rpartition(".")
                if suffix != "png" or not stem.isdecimal() or int(stem) not in missing:
                    continue
                gfx_id = int(stem)
                textures[gfx_id] = _read_texture(asset, size)
                missing.remove(gfx_id)
            if not missing:
                break
        if missing:
            raise FileNotFoundError(f"Missing room textures in {content / 'Map' / 'Textures'}, group {bucket}: {sorted(missing)}")
    return textures


def _stamp(element, textures):
    source = textures[element["gfxId"]]
    sx, sy = element["scale"]["x"], element["scale"]["y"]
    if sx == 0 or sy == 0:
        return None
    if sx < 0:
        source = source.transpose(Image.Transpose.FLIP_LEFT_RIGHT)
    if sy < 0:
        source = source.transpose(Image.Transpose.FLIP_TOP_BOTTOM)
    size = (max(1, round(source.width * abs(sx))), max(1, round(source.height * abs(sy))))
    if size != source.size:
        source = source.resize(size, Image.Resampling.BICUBIC)
    color = element["color"]["value"]
    # Map tints store neutral at 128 for RGB, and full opacity at 255.
    gains = [((color >> shift) & 255) / 128 for shift in (16, 8, 0)] + [(color >> 24 & 255) / 255]
    source = source.point([min(255, round(value * gain)) for gain in gains for value in range(256)])
    if element["rotation"]:
        source = source.rotate(-element["rotation"], resample=Image.Resampling.BICUBIC, expand=True)
    at = element["position"]
    return source, round(at["x"] - source.width / 2), round(-at["y"] - source.height / 2)


def _composite(canvas, source, x, y, blend):
    if not blend:
        canvas.paste(source, (x, y), source.getchannel("A"))
        return
    dest = canvas.crop((x, y, x + source.width, y + source.height))
    rgb = source.convert("RGB")
    factors = (blend["sourceFactor"], blend["destinationFactor"])
    if factors == (2, 10):  # DstColor, OneMinusSrcAlpha
        result = ImageChops.multiply(dest, rgb)
    elif factors == (2, 1):  # DstColor, One
        result = ImageChops.add(dest, ImageChops.multiply(dest, rgb))
    elif factors == (3, 1):  # SrcColor, One
        result = ImageChops.add(dest, ImageChops.multiply(rgb, rgb))
    else:
        raise ValueError(f"Unsupported map blend factors {factors}")
    canvas.paste(result, (x, y), source.getchannel("A"))


def _draw(data, textures):
    stamps = []
    for element, blend in _render_elements(data):
        stamp = _stamp(element, textures)
        if stamp:
            stamps.append((*stamp, blend))
    x0 = min(x for _, x, _, _ in stamps)
    y0 = min(y for _, _, y, _ in stamps)
    width = max(x + image.width for image, x, _, _ in stamps) - x0
    height = max(y + image.height for image, _, y, _ in stamps) - y0
    background = data["backgroundColor"]["value"]
    canvas = Image.new("RGB", (width, height), tuple((background >> shift) & 255 for shift in (16, 8, 0)))
    for image, x, y, blend in stamps:
        _composite(canvas, image, x - x0, y - y0, blend)
    # The source scenes include oversized black lighting masks. Keeping their
    # transparent margins in the camera fit would shrink the room to a thumbnail.
    bounds = canvas.convert("L").point(lambda value: 255 if value > 2 else 0).getbbox()
    if bounds is None:
        raise ValueError(f"Map {data['id']} rendered empty")
    return canvas.crop(bounds), x0 + bounds[0], y0 + bounds[1]


def _grid(data, image, left, top):
    # A DOFUS map numbers 40 staggered rows of 14 cells. Combat uses the
    # perpendicular axes of the diamond, trimmed to the room's playable extent.
    grid = {}
    for cell in data["cellsData"]:
        row, column = divmod(cell["cellNumber"], 14)
        grid[column + (row + 1) // 2, row // 2 - column] = cell
    walkable = [xy for xy, cell in grid.items() if cell["mov"]]
    xs, ys = zip(*walkable)
    first_x, first_y = min(xs), min(ys)
    width, height = max(xs) - first_x + 1, max(ys) - first_y + 1
    spec = {
        "id": data["id"], "width": width, "height": height,
        "imageWidth": image.width, "imageHeight": image.height,
        "originX": HALF_W * (first_x - first_y - 13.5) - left,
        "originY": HALF_H * (first_x + first_y - 22.5) - top,
        "halfCellWidth": HALF_W, "halfCellHeight": HALF_H,
        "walls": [], "sightWalls": [], "red": [], "blue": [],
    }
    for tile in range(width * height):
        cell = grid.get((first_x + tile % width, first_y + tile // width), {})
        if not cell.get("mov"):
            spec["walls"].append(tile)
        if not cell.get("los"):
            spec["sightWalls"].append(tile)
        for side in ("red", "blue"):
            if cell.get(side) and cell.get("mov"):
                spec[side].append(tile)
    return spec


def export_maps(content: Path, output: Path):
    if not UnityPy.config.FALLBACK_UNITY_VERSION:
        UnityPy.config.FALLBACK_UNITY_VERSION = _client_unity_version(content)
    maps = []
    for map_id in (VILLAGE, ASTRUB):
        data = _read_map(content, map_id)
        if data is not None:
            maps.append(data)
        else:
            print(f"Map {map_id} is absent from {content / 'Map' / 'Data'}", file=sys.stderr)
    if not maps:
        raise FileNotFoundError("The installed DOFUS client has neither training room.")
    textures = _read_textures(content, {e["gfxId"] for data in maps for e, _ in _render_elements(data)})
    target = output / "maps"
    target.mkdir(parents=True, exist_ok=True)
    for data in maps:
        image, left, top = _draw(data, textures)
        spec = _grid(data, image, left, top)
        image.save(target / f"{data['id']}.webp", lossless=True, method=4)
        (target / f"{data['id']}.json").write_text(json.dumps(spec, separators=(",", ":")), encoding="utf-8")
