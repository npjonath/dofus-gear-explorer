import tempfile
import unittest
from pathlib import Path

from effects import old_sequences, timing
from hud import addressables_directory


class CombatAssetsTest(unittest.TestCase):
    def test_hud_in_macos_app_and_in_standalone_players(self):
        layouts = (
            ("DofusContent/Content", "Dofus.app/Contents/Resources/Data/StreamingAssets/aa/StandaloneOSX"),
            ("Dofus_Data/StreamingAssets/Content", "Dofus_Data/StreamingAssets/aa/StandaloneWindows64"),
            ("DofusContent/Content", "Dofus_Data/StreamingAssets/aa/StandaloneLinux64"),
        )
        for relative_content, relative_aa in layouts:
            with self.subTest(aa=relative_aa), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                content, aa = root / relative_content, root / relative_aa
                content.mkdir(parents=True)
                aa.mkdir(parents=True)
                (aa / "figs_assets_2x.bundle").touch()
                self.assertEqual(addressables_directory(content), aa)

    def test_legacy_spell_script_keeps_both_target_graphics(self):
        sequences = old_sequences("animId:2,targetGfxId:10,targetGfxId2:11,targetGfxShowUnder2:1")
        self.assertEqual(sequences, [
            {"gfxs": [{"type": "cast", "casterAnimation": "AnimAttaque2"}]},
            {"gfxs": [
                {"type": "target", "gfxId": 10},
                {"type": "target", "gfxId": 11, "strata": "background"},
            ]},
        ])

    def test_timing_uses_the_shot_label_after_padded_labels(self):
        data = bytes((30, 0, 1, 0, 2, 0, 0, 0, 1, 0, 3)) + b"one" + bytes((12, 0, 4)) + b"SHOT"
        self.assertEqual(timing(data, 30), {"duration": 1000, "shot": 400})
