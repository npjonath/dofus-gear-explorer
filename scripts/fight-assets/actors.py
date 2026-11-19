"""Export DOFUS character atlases, skeletal clips and wearable appearances for WebGL."""

import concurrent.futures
import gzip
import hashlib
import json
import math
import os
import re
from pathlib import Path

import UnityPy
from UnityPy.export.Texture2DConverter import get_image_from_texture2d


# These three item/skin pairs have been checked on an equipped character against
# the client artwork. Ordinary hats, capes and shields currently have appearanceId=0.
VERIFIED_SKINS = {32201: 5740, 32207: 5741, 33596: 5830}
VISIBLE_TYPES = {16, 17, 82}  # hat, cape and shield
POUTCH_INGBALL_ID = 494


def _json(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    raw = json.dumps(data, separators=(",", ":"), allow_nan=False).encode()
    if path.name == "skin.json":
        path.with_suffix(".json.gz").write_bytes(gzip.compress(raw, compresslevel=3, mtime=0))
    else:
        path.write_bytes(raw)


def _root(bundle):
    env = UnityPy.load(str(bundle))
    return next(iter(env.container.values())).deref()


def read_data(content, name):
    root = _root(content / "Data" / f"data_assets_{name}dataroot.asset.bundle").parse_as_dict()
    objects = root["objectsById"]
    refs = {entry["rid"]: entry["data"] for entry in root["references"]["RefIds"]}
    return {key: refs[value["rid"]] for key, value in zip(objects["m_keys"], objects["m_values"], strict=True)}


def _skin(obj, directory):
    skin = obj.parse_as_dict()
    directory.mkdir(parents=True, exist_ok=True)
    for i, ref in enumerate(skin["textures"]):
        texture = obj.assets_file.objects[ref["m_PathID"]].read()
        # The runtime uses the game's UV origin; Pillow's default UnityPy flip is wrong here.
        image = get_image_from_texture2d(texture, False)
        image.save(directory / f"{i}.webp", lossless=True, method=0, exact=True)
    skin["textures"] = [{} for _ in skin["textures"]]
    for key in ("m_GameObject", "m_Script"):
        skin.pop(key, None)
    _json(directory / "skin.json", skin)


def export_bone(bundle: Path, directory: Path):
    """Export an actor or spell effect bone, its intrinsic artwork, and each animation."""
    obj = _root(bundle)
    bone = obj.parse_as_dict()
    objects = obj.assets_file.objects
    _skin(objects[bone.pop("boneAsset")["m_PathID"]], directory)
    clips = []
    for animation in bone["animations"]:
        animation["bounds"] = {k: v if math.isfinite(v) else None for k, v in animation["bounds"].items()}
        name = animation["name"]
        if not re.fullmatch(r"[\w. ~-]+", name) or name.endswith((".", " ")):
            continue
        pointer = animation.pop("data")
        data = animation.pop("dataBytes", None)
        if data is None:
            # UnityPy preserves arbitrary TextAsset bytes through surrogateescape.
            script = objects[pointer["m_PathID"]].parse_as_dict()["m_Script"]
            data = script.encode("utf-8", "surrogateescape") if isinstance(script, str) else script
        (directory / f"{name}.dat").write_bytes(bytes(data))
        clips.append(animation)
    bone["animations"] = clips
    for graphic in bone["graphics"]:
        graphic.pop("asset", None)
    for key in ("m_GameObject", "m_Script"):
        bone.pop(key, None)
    _json(directory / "bone.json", bone)
    return {m.group(1) for animation in bone["animations"]
            if (m := re.fullmatch(r"(.+)_([0-7])", animation["name"]))}


def _appearance_file(cache):
    return cache / "equipment-skins.json"


def _override_skins(raw):
    document = json.loads(raw)
    pairs = document.get("skins", document) if isinstance(document, dict) else None
    if not isinstance(pairs, dict) or any(
        not key.isascii() or not key.isdigit() or int(key) <= 0 or type(value) is not int or value <= 0
        for key, value in pairs.items()
    ):
        raise ValueError("equipment-skins.json must map positive item IDs to positive skin IDs.")
    return {int(key): value for key, value in pairs.items()}


def prepare_equipment_catalog(cache: Path) -> str:
    """Include local appearance corrections in the immutable pack identity."""
    path = _appearance_file(cache)
    if not path.is_file():
        return ""
    raw = path.read_bytes()
    _override_skins(raw)
    return hashlib.sha256(raw).hexdigest()


def _equipment(cache, items, sources):
    result = dict(VERIFIED_SKINS)
    overrides = _appearance_file(cache)
    if overrides.is_file():
        result.update(_override_skins(overrides.read_bytes()))
    for key, item in items.items():
        if item["typeId"] in VISIBLE_TYPES and item["appearanceId"] > 0:
            skin = item["appearanceId"]
            if (sources / f"skins_assets_skin_{skin}.bundle").is_file():
                result[key] = skin
    return {key: skin for key, skin in result.items() if key in items and
            (sources / f"skins_assets_skin_{skin}.bundle").is_file()}


def export_actors(content: Path, output: Path):
    UnityPy.config.FALLBACK_UNITY_VERSION = UnityPy.config.FALLBACK_UNITY_VERSION or "6000.0.44f1"
    actors = output / "actors"
    source = content / "Characters"
    dest = actors / "Content" / "Characters"
    breeds = read_data(content, "breeds")
    bodies = read_data(content, "bodies")
    heads = read_data(content, "heads")
    rules = read_data(content, "skinslotsrules")
    items = read_data(content, "items")
    for name, rows in (("breeds", breeds), ("bodies", bodies), ("skinslotsrules", rules)):
        _json(actors / "Content" / "Data" / f"{name}dataroot.json", {"objectsById": rows})

    # The stuff has no player-defined appearance. Use the class's first creation head.
    looks = {}
    skins = set()
    for breed, row in breeds.items():
        head = min((h for h in heads.values() if h["breed"] == breed and h["gender"] == 0
                    and h["availableAtCreation"]), key=lambda h: (h["order"], h["id"]), default=None)
        parts = row["maleLook"].strip("{}").split("|")
        if head:
            parts[1] += "," + str(head["skins"])
        looks[breed] = "{" + "|".join(parts) + "}"
        skins.update(int(s) for s in parts[1].split(",") if s)
    equipment = _equipment(output.parent, items, source / "Skins")
    skins.update(equipment.values())
    weapon_types = {key: row["typeId"] for key, row in items.items() if "criticalHitBonus" in row}
    from creatures import summon_catalog

    monsters = read_data(content, "monsters")
    creatures = summon_catalog(content, monsters)
    _json(actors / "creatures.json", creatures)
    creature_looks = {key: row["look"] for key, row in creatures.items()}
    dummy_look = monsters[POUTCH_INGBALL_ID]["look"]
    dummy_bone = dummy_look.strip("{}").split("|")[0]
    bone_names = {"1-static", "1-movement", "1-combat", "1-combat-armes", dummy_bone}
    for look in creature_looks.values():
        parts = look.strip("{}").split("|")
        bone_names.add(parts[0])
        if len(parts) > 1:
            skins.update(int(s) for s in parts[1].split(",") if s.isdecimal())
    for breed in looks:
        bone_names.update((f"1-{breed}-static", f"1-{breed}-combat"))
    sequences = {}

    for name in sorted(bone_names):
        path = source / "Bones" / f"bones_assets_bone_{name}.bundle"
        if not path.exists():
            if name.startswith("1-") and name.split("-")[1].isdigit():
                continue
            raise FileNotFoundError(path)
        names = export_bone(path, dest / "Bones" / name)
        sequences[name] = sorted(names)

    def export_wearable(skin):
        obj = _root(source / "Skins" / f"skins_assets_skin_{skin}.bundle")
        _skin(obj, dest / "Skins" / str(skin))

    with concurrent.futures.ThreadPoolExecutor(max_workers=min(4, os.cpu_count() or 1)) as pool:
        for _ in pool.map(export_wearable, sorted(skins)):
            pass

    _json(actors / "index.json", {"breeds": looks, "dummy": dummy_look, "creatures": creature_looks, "equipment": equipment,
                                  "weapons": weapon_types, "animations": sequences})
