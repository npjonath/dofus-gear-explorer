"""Read the summon grades and multilingual labels reachable from player spells."""

import struct

from actors import read_data
from effects import castable_spells


SUMMON_ACTIONS = {181, 405, 1008, 1011, 2796}
CAST_ACTIONS = {400, 401, 402, 792, 1017, 1018, 1019, 1091, 1160, 1165, 2022, 2160, 2794, 2960}
CHARACTERISTICS = {
    "lifePoints": 0, "actionPoints": 1, "strength": 10, "vitality": 11,
    "wisdom": 12, "chance": 13, "agility": 14, "intelligence": 15,
    "damageBonus": 16, "criticalHitBonus": 18, "rangeBonus": 19, "movementPoints": 23,
    "percentDamageBonus": 25, "maxSummon": 26, "paLostDodge": 27, "mpLostDodge": 28,
    "reductionEarth": 33, "reductionFire": 34, "reductionWater": 35, "reductionAir": 36, "reductionNeutral": 37,
    "initiativeBonus": 44, "healBonus": 49, "reductionEarthFlat": 54, "reductionFireFlat": 55,
    "reductionWaterFlat": 56, "reductionAirFlat": 57, "reductionNeutralFlat": 58,
    "trapDamageBonus": 70, "trapDamageBonusPercent": 69, "tackleEvade": 78, "tackleBonus": 79,
    "apAttack": 82, "mpAttack": 83, "pushDamageBonus": 84, "pushDamageReduction": 85,
    "criticalDamageBonus": 86, "criticalDamageReduction": 87, "earthDamageBonus": 88,
    "fireDamageBonus": 89, "waterDamageBonus": 90, "airDamageBonus": 91, "neutralDamageBonus": 92,
}


def _translations(path, ids):
    if not path.is_file():
        return {}
    data = path.read_bytes()
    offset = 1 + data[0]
    count, = struct.unpack_from("<I", data, offset)
    result = {}
    for key, pos in struct.iter_unpack("<II", data[offset + 4:offset + 4 + 8 * count]):
        if key in ids:
            size, shift = 0, 0
            while True:
                b = data[pos]
                pos += 1
                size |= (b & 127) << shift
                if b < 128:
                    break
                shift += 7
            result[key] = data[pos:pos + size].decode("utf-8")
    return result


def _stats(grade):
    return {stat: grade[name] for name, stat in CHARACTERISTICS.items() if grade.get(name)}


def _spells(monster, grade):
    spells = []
    for spell, config in zip(monster["spells"], monster["spellGrades"]):
        entries = config.split(";")
        if 0 < grade <= len(entries):
            level = entries[grade - 1].split(",")[0]
            if level.isdecimal() and int(level):
                spells.append({"id": spell, "grade": int(level)})
    return spells


def summon_catalog(content, monsters):
    levels = read_data(content, "spelllevels")
    spells = read_data(content, "spells")
    check = list(castable_spells(content))
    visited = set()
    creatures = {}
    while check:
        spell = check.pop()
        if spell in visited or spell not in spells:
            continue
        visited.add(spell)
        for grade in spells[spell]["spellLevels"]:
            if grade not in levels:
                continue
            for group in ("effects", "criticalEffect"):
                for effect in levels[grade][group]:
                    action, child = effect["actionId"], effect["diceNum"]
                    if action in CAST_ACTIONS:
                        check.append(child)
                    if action not in SUMMON_ACTIONS or child in creatures or child not in monsters:
                        continue
                    monster = monsters[child]
                    grades = []
                    for g in monster["grades"]:
                        playable = _spells(monster, g["grade"])
                        check.extend(s["id"] for s in playable)
                        opening = levels.get(g["startingSpellId"], {})
                        opening_spell = opening.get("spellId", 0)
                        if opening_spell:
                            check.append(opening_spell)
                        bonus = g["bonusCharacteristics"]
                        grades.append({"grade": g["grade"], "level": g["level"], "hp": g["lifePoints"],
                                       "ap": g["actionPoints"], "mp": g["movementPoints"],
                                       "bonusHp": bonus["lifePoints"], "stats": _stats(g), "bonuses": _stats(bonus),
                                       "spells": playable, "startingSpellId": opening_spell,
                                       "startingSpellGrade": opening.get("grade", 0)})
                    creatures[child] = {
                        "names": {}, "look": monster["look"], "static": not bool(monster["m_flags"] & 64),
                        "summonCost": monster["summonCost"], "usesSummonSlot": bool(monster["m_flags"] & 1),
                        "ratios": {int(r["values"][0]): r["values"][1] for r in monster["characRatios"]},
                        "grades": grades,
                    }
    names = {int(monsters[creature]["nameId"]) for creature in creatures}
    for lang in ("fr", "en", "es", "pt"):
        translations = _translations(content / "I18n" / f"{lang}.bin", names)
        for creature, entry in creatures.items():
            label = translations.get(int(monsters[creature]["nameId"]))
            if label:
                entry["names"][lang] = label
    return creatures
