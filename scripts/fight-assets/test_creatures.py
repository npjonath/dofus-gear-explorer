import struct
import tempfile
import unittest
from pathlib import Path

from creatures import _spells, _stats, _translations


class SummonGradeTests(unittest.TestCase):
    def test_each_spell_uses_the_grade_for_that_summon(self):
        cat = {"spells": [42, 198], "spellGrades": ["1,1;1,1;1,1", "1,1;2,1;3,1"]}
        self.assertEqual(_spells(cat, 2), [{"id": 42, "grade": 1}, {"id": 198, "grade": 2}])
        self.assertEqual(_spells(cat, 5), [])

    def test_life_and_combat_stats_use_the_same_ids_as_the_combat_engine(self):
        grade = {"lifePoints": 66, "actionPoints": 6, "movementPoints": 4, "reductionFire": 15}
        self.assertEqual(_stats(grade), {0: 66, 1: 6, 23: 4, 34: 15})

    def test_game_language_table_offsets_are_byte_offsets(self):
        labels = [(11, "Chaton Enragé"), (93, "Lanterne")]
        # Language prefix, number of ID entries, their byte offsets, number of named UI entries.
        start = 3 + 4 + len(labels) * 8 + 4
        ids = bytearray()
        texts = bytearray()
        for key, label in labels:
            raw = label.encode()
            ids.extend(struct.pack("<II", key, start + len(texts)))
            texts.extend(bytes([len(raw)]) + raw)
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "fr.bin"
            path.write_bytes(b"\2fr" + struct.pack("<I", len(labels)) + ids + bytes(4) + texts)
            self.assertEqual(_translations(path, {11}), {11: "Chaton Enragé"})


if __name__ == "__main__":
    unittest.main()
