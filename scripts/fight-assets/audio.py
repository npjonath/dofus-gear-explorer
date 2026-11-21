"""Export combat sounds assigned by the client's map playlists and animation events."""
import hashlib
import json
import re
import unicodedata
from collections import defaultdict
from pathlib import Path

import UnityPy

from actors import read_data
from combat_bank import Bank, remux
from creatures import _translations
from hud import addressables_directory
from maps import _read_map


UI_EVENTS = {
    "begin": "Fight_Start", "endTurn": "Fight_PasserTour", "turn": "Fight_TourSuivant_Perso",
    "click": "Bouton_Générique", "open": "Sort_Ouverture", "close": "Sort_Fermeture",
    "critical": "Fight_CoupCritique", "move": "Fight_CaseDéplacement",
}


def normalized(name):
    return "".join(c.lower() for c in unicodedata.normalize("NFKD", name) if c.isascii() and c.isalnum())


def animation_events(content, output, allowed):
    bones = read_data(content, "soundbones")
    actors = json.loads((output / "actors/index.json").read_bytes())
    graphics = json.loads((output / "effects/spells.json").read_bytes())["clips"]
    result = {}
    game_bones = output / "actors/Content/Characters/Bones"
    for directory in (d for d in game_bones.iterdir() if d.is_dir()):
        name = directory.name
        if not name.split("-")[0].isdecimal():
            continue
        id_ = int(name.split("-")[0])
        sound = bones.get(id_, {}).get("animSounds")
        if not sound:
            continue
        graphics_entry = graphics.get(name)
        clips = set(actors["animations"].get(name, []))
        if graphics_entry:
            clips.update([graphics_entry["animation"], graphics_entry.get("idle", ""), graphics_entry.get("trigger", {}).get("animation", "")])
        if not clips:
            continue
        fps = json.loads((directory / "bone.json").read_bytes()).get("defaultFrameRate") or 30
        for key, props in zip(sound["m_keys"], sound["m_values"]):
            bone, slash, clip = key.rpartition("/")
            if slash and bone.casefold() != name.casefold() or clip not in clips:
                continue
            cues = []
            for guid, frames in zip(props["guids"], props["startFrames"]):
                if guid not in allowed:
                    continue
                for frame in frames.split("|"):
                    if frame.isdecimal():
                        cues.append([round(max(0, int(frame) - 1) * 1000 / fps), guid])
            if cues:
                result.setdefault(name, {})[clip] = cues
    return result


def export_audio(content: Path, output: Path):
    destination = output / "audio"
    destination.mkdir(parents=True, exist_ok=True)
    index = {"maps": {}, "animations": {}, "events": {}, "voices": {}, "ui": {}, "poutchHit": [], "categories": []}
    bank_root = next((p.parent for p in (content / "Audio/Banks").glob("*/Master.bank")), None)
    manager = addressables_directory(content) / "gameassets_assets_all.bundle"
    if bank_root and manager.is_file():
        library = UnityPy.load(str(manager)).container["Assets/Configuration/Audio/AudioManagerLibrary.asset"].deref_parse_as_dict()
        populate(index, content, output, destination, bank_root, library)
    (destination / "index.json").write_text(json.dumps(index, separators=(",", ":")), encoding="utf-8")


def populate(index, content, output, dest, bank_root, library):
    files = dest / "samples"
    files.mkdir(exist_ok=True)
    events = {e["guid"]: e for e in library["m_eventInfoSet"]["m_entries"]}
    paths = {e["path"]: guid for guid, e in events.items()}
    banks = library["m_bankInfoSet"]["m_entries"]
    relations = library["m_eventsBanksRelations"]
    index["animations"] = animation_events(content, output, events)
    roles = {}
    for animations in index["animations"].values():
        for cues in animations.values():
            for _, guid in cues:
                roles[guid] = "voices" if "/Barks/" in events[guid]["path"] else "effects"
    for key, event in UI_EVENTS.items():
        guid = paths.get("event:/Ui/" + event)
        if guid:
            roles[guid] = "ui"
            index["ui"][key] = guid
    for map_path in sorted((output / "maps").glob("*.json")):
        map_id = int(map_path.stem)
        data = _read_map(content, map_id)
        if not data:
            continue
        entry = {}
        for key, role in (("musicPlaylist", "music"), ("ambiantPlaylist", "ambience")):
            guid = data["playlistSet"].get(key, {}).get("fmodEventGuid")
            if guid in events:
                roles[guid] = role
                entry[role] = guid
        index["maps"][map_id] = entry

    # Store only event-linked samples reachable in this combat pack.
    requests = defaultdict(set)
    for guid in roles:
        event = events[guid]
        start = event["relationalBankIndex"]
        for relation in relations[start:start + event["relationalBankCount"]]:
            if 0 <= relation < len(banks):
                stem = banks[relation]["path"].rsplit("/", 1)[-1]
                if re.fullmatch(r"[\w -]+", stem):
                    requests[stem].add(guid)
    actors = json.loads((output / "actors/index.json").read_bytes())
    dummy_bone = actors["dummy"].lstrip("{").split("|")[0].rstrip("}")
    index["dummy"] = dummy_bone
    index["creatures"] = {key: look.lstrip("{").split("|")[0].rstrip("}") for key, look in actors.get("creatures", {}).items()}
    index["weapons"] = actors.get("weapons", {})
    requests[dummy_bone]
    breeds = read_data(content, "breeds")
    french = _translations(content / "I18n/fr.bin", {int(v["shortNameId"]) for v in breeds.values()})
    breed_ids = {normalized(french[int(v["shortNameId"])]): i for i, v in breeds.items() if int(v["shortNameId"]) in french}
    present = set()

    for stem, requested in sorted(requests.items()):
        path = bank_root / (stem + ".bank")
        if not path.is_file():
            continue
        with Bank(path) as bank:
            cache = {}
            def store(ref):
                if ref not in cache:
                    sample = bank.sample_for(ref)
                    encoded = remux(bank.data, sample) if sample else None
                    cache[ref] = None
                    if encoded:
                        name = hashlib.sha256(encoded).hexdigest()[:24]
                        file = files / f"{name}.ogg"
                        if not file.is_file():
                            file.write_bytes(encoded)
                        cache[ref] = name
                return cache[ref]

            for guid in sorted(requested):
                if guid in index["events"]:
                    continue
                role = roles[guid]
                timeline = []
                for time, refs in bank.clips(guid):
                    variants = defaultdict(set)
                    local = set()
                    for ref in refs:
                        sample = bank.sample_for(ref)
                        if not sample:
                            continue
                        # The UI's training fight uses the ordinary soundtrack; the 8-bit track
                        # is an alternative parameter branch in the same FMOD music event.
                        if role == "music" and "8bit" in sample.name.lower():
                            continue
                        breed = None
                        if role == "voices":
                            parts = sample.name.split("_")
                            if len(parts) < 5 or parts[0:2] != ["Avatar", "Bark"] or parts[3] != "Homme":
                                continue
                            breed = breed_ids.get(normalized(parts[2]))
                            if breed is None:
                                continue
                        asset = store(ref)
                        if asset and breed:
                            variants[breed].add(asset)
                        elif asset:
                            local.add(asset)
                    if variants:
                        for breed, names in variants.items():
                            take = index["voices"].setdefault(breed, {}).setdefault(guid, [])
                            take.extend(sorted(names - set(take)))
                    if local:
                        timeline.append([round(time * 1000), sorted(local)])
                if timeline or role == "voices" and any(guid in row for row in index["voices"].values()):
                    index["events"][guid] = {"channel": role, "layers": timeline}
                    present.add(role)
            if stem == dummy_bone:
                for bank_index, samples in enumerate(bank.samples):
                    for offset, sample in enumerate(samples):
                        if "ingballanimhit" in normalized(sample.name):
                            asset = store((bank_index, offset))
                            if asset:
                                index["poutchHit"].append(asset)
                                present.add("effects")
    index["categories"] = sorted(present)
