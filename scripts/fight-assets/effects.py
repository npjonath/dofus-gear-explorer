"""Translate DOFUS spell animation scripts and extract their local sprite bones."""

import json
import re
import struct
from pathlib import Path

import UnityPy

from actors import export_bone, read_data


def castable_spells(content):
    breeds = read_data(content, "breeds")
    classes = {spell for breed in breeds.values() for spell in breed["breedSpellsId"]}
    result = set(classes)
    for variant in read_data(content, "spellvariants").values():
        pair = variant["spellIds"]
        if len(pair) == 2 and (variant["breedId"] not in breeds or pair[0] in classes):
            result.update(pair)
    return result


def fight_spells(content, output, spells):
    from creatures import CAST_ACTIONS

    playable = castable_spells(content)
    source = output / "actors/creatures.json"
    if source.exists():
        for creature in json.loads(source.read_bytes()).values():
            for grade in creature["grades"]:
                playable.update(s["id"] for s in grade["spells"])
                if grade["startingSpellId"]:
                    playable.add(grade["startingSpellId"])
    levels = read_data(content, "spelllevels")
    casts = CAST_ACTIONS | {1091, 1165}
    pending = list(playable)
    while pending:
        spell = spells.get(pending.pop())
        if not spell:
            continue
        for level_id in spell["spellLevels"]:
            level = levels.get(level_id)
            if level:
                for effect in (*level["effects"], *level["criticalEffect"]):
                    child = effect["diceNum"]
                    if effect["actionId"] in casts and child in spells and child not in playable:
                        playable.add(child)
                        pending.append(child)
    return playable


def old_sequences(parameters):
    values = dict(pair.split(":", 1) for pair in parameters.split(",") if ":" in pair)
    sequences = []
    animation = values.get("animId", "")
    if animation.isdecimal():
        sequences.append({"gfxs": [{"type": "cast", "casterAnimation": "AnimAttaque" + animation}]})
    for parts in ((("caster", "caster"),), (("missile", "projectile"), ("trail", "trail")),
                  (("target", "target"), ("foreground", "target"), ("glyph", "mark"))):
        gfxs = []
        for prefix, kind in parts:
            for suffix in ("", "2"):
                bone = values.get(f"{prefix}GfxId{suffix}", "")
                if not bone.isdecimal() or int(bone) == 0:
                    continue
                gfx = {"type": kind, "gfxId": int(bone)}
                if values.get(f"{prefix}GfxShowUnder{suffix}") == "1":
                    gfx["strata"] = "background"
                gfxs.append(gfx)
        if gfxs:
            sequences.append({"gfxs": gfxs})
    return sequences


def read_script(parameters):
    if not parameters.startswith("{"):
        return old_sequences(parameters)
    try:
        return json.loads(parameters).get("main", {}).get("sequences", [])
    except (ValueError, AttributeError):
        return []


def bindings(entries, scripts):
    result = []
    for entry in sorted(entries, key=lambda item: item["order"]):
        script = scripts.get(entry["scriptId"])
        if not script:
            continue
        cast = {}
        steps = []
        for step in read_script(script["rawParams"]):
            graphics = []
            for raw in step.get("gfxs", []):
                kind = raw.get("type")
                if kind == "cast":
                    if raw.get("casterAnimation"):
                        cast["cast"] = raw["casterAnimation"]
                elif kind in {"caster", "target", "trail", "projectile", "mark"} and raw.get("gfxId"):
                    graphic = {"kind": kind, "bone": raw["gfxId"]}
                    for key in ("strata", "scaleX", "scaleY", "offsetY", "curvature", "isOriented"):
                        if key in raw:
                            graphic[key] = raw[key]
                    graphics.append(graphic)
            if graphics:
                steps.append({"gfx": graphics, "end": step.get("endPolicy", "waitForAll")})
        if not cast and not steps:
            continue
        if steps:
            cast["sequences"] = steps
        for key in ("casterMask", "targetMask", "activationMask", "criterion", "random", "spellLevels"):
            if entry.get(key):
                cast[key] = entry[key]
        result.append(cast)
    return result


def timing(data, fps):
    frames, _, labels = struct.unpack_from("<HHH", data)
    cursor = 8
    shot = None
    for _ in range(labels):
        frame, length = struct.unpack_from("<HB", data, cursor)
        label = data[cursor + 3:cursor + 3 + length]
        if label == b"SHOT":
            shot = frame
            break
        cursor = (cursor + 3 + length + 1) & ~1
    duration = max(1, round(frames * 1000 / fps))
    result = {"duration": duration}
    if shot is not None:
        result["shot"] = round(shot * 1000 / fps)
    return result


def casting_times(content, animations):
    result = {}
    directory = content / "Characters/Bones"
    # Common gestures live on 1-combat; class-specific attack numbers live
    # on the class's own skeleton. All directions of a cast share the SHOT.
    files = sorted(directory.glob("bones_assets_bone_1-*-combat.bundle"))
    files.extend(directory / name for name in ("bones_assets_bone_1-combat.bundle", "bones_assets_bone_1-combat-armes.bundle"))
    for bundle in files:
        if not bundle.is_file():
            continue
        env = UnityPy.load(str(bundle))
        obj = next(iter(env.container.values())).deref()
        bone = obj.parse_as_dict()
        fps = bone.get("defaultFrameRate") or 30
        for clip in bone["animations"]:
            name = re.sub(r"_[0-7]$", "", clip["name"])
            if name not in animations or name in result:
                continue
            data = obj.assets_file.objects[clip["data"]["m_PathID"]].read().m_Script
            result[name] = timing(data.encode("utf-8", "surrogateescape"), fps)
    return result


def export_effects(content: Path, output: Path, spell_ids=None) -> None:
    spells = read_data(content, "spells")
    scripts = read_data(content, "spellscripts")
    catalog = {}
    bone_ids = set()
    animations = set()
    for spell_id in sorted(fight_spells(content, output, spells) if spell_ids is None else spell_ids):
        if spell_id not in spells:
            continue
        spell = spells[spell_id]
        normal = bindings(spell["boundScriptUsageData"], scripts)
        critical = bindings(spell["criticalHitBoundScriptUsageData"], scripts)
        if not normal and not critical:
            continue
        catalog[spell_id] = {"normal": normal}
        if critical != normal:
            catalog[spell_id]["critical"] = critical
        for cast in normal + critical:
            if cast.get("cast"):
                animations.add(cast["cast"])
            for step in cast.get("sequences", []):
                bone_ids.update(g["bone"] for g in step["gfx"])

    clips = {}
    for bone_id in sorted(bone_ids):
        bundle = content / "Characters/Bones" / f"bones_assets_bone_{bone_id}.bundle"
        if not bundle.is_file():
            continue
        directory = output / "actors/Content/Characters/Bones" / str(bone_id)
        if not (directory / "bone.json").is_file():
            export_bone(bundle, directory)
        bone = json.loads((directory / "bone.json").read_bytes())
        animation = next((anim for anim in bone["animations"] if anim["name"].startswith("FX_")), None)
        if animation is None and bone["animations"]:
            animation = bone["animations"][0]
        if animation:
            name = animation["name"]
            clips[bone_id] = timing((directory / (name + ".dat")).read_bytes(), bone.get("defaultFrameRate") or 30)
            clips[bone_id]["animation"] = re.sub(r"_[0-7]$", "", name)
            clips[bone_id]["directions"] = [int(a["name"][-1]) for a in bone["animations"]
                                             if a["name"][:-1] == name[:-1] and a["name"][-1] in "01234567"]
            idle = next((a["name"] for a in bone["animations"] if re.fullmatch(r"AnimStatique_[0-7]", a["name"])), None)
            if idle:
                clips[bone_id]["idle"] = idle
                trigger = next((a["name"] for a in bone["animations"] if re.fullmatch(r"AnimTrigger_[0-7]", a["name"])), None)
                if trigger:
                    trigger_time = timing((directory / (trigger + ".dat")).read_bytes(), bone.get("defaultFrameRate") or 30)
                    clips[bone_id]["trigger"] = {"animation": trigger, **trigger_time}

    folder = output / "effects"
    folder.mkdir(parents=True, exist_ok=True)
    (folder / "spells.json").write_text(
        json.dumps({"spells": catalog, "clips": clips, "casts": casting_times(content, animations)}, separators=(",", ":")),
        encoding="utf-8"
    )
