import gzip
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import actors


class EquipmentCatalogTests(unittest.TestCase):
    def test_without_a_local_table_only_available_verified_skins_are_used(self):
        with tempfile.TemporaryDirectory() as temp:
            cache = Path(temp)
            item = {"typeId": 16, "appearanceId": 0}
            (cache / "skins_assets_skin_5740.bundle").touch()
            self.assertEqual(actors.prepare_equipment_catalog(cache), "")
            self.assertEqual(actors._equipment(cache, {32201: item, 120: item}, cache), {32201: 5740})

    def test_local_table_changes_the_pack_id_and_game_data_wins_when_available(self):
        raw = b'{"skins":{"3":17, "4":2, "5":19}}'
        with tempfile.TemporaryDirectory() as temp:
            cache = Path(temp)
            (cache / "equipment-skins.json").write_bytes(raw)
            for skin in (2, 17, 21):
                (cache / f"skins_assets_skin_{skin}.bundle").touch()
            items = {3: {"typeId": 16, "appearanceId": 0}, 4: {"typeId": 17, "appearanceId": 21},
                     5: {"typeId": 82, "appearanceId": 0}}
            self.assertEqual(actors.prepare_equipment_catalog(cache), hashlib.sha256(raw).hexdigest())
            self.assertEqual(actors._equipment(cache, items, cache), {3: 17, 4: 21})

    def test_invalid_mapping_has_an_actionable_error(self):
        with self.assertRaisesRegex(ValueError, "positive item IDs"):
            actors._override_skins(b'{"3":"relative/file"}')

    def test_atlas_geometry_round_trips_without_game_files(self):
        data = {"triangles": [2, 1, 0], "textures": [{}]}
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "skin.json"
            actors._json(path, data)
            self.assertEqual(json.loads(gzip.decompress(path.with_suffix(".json.gz").read_bytes())), data)
            self.assertFalse(path.exists())


if __name__ == "__main__":
    unittest.main()
