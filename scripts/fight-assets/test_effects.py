import tempfile
import unittest
from pathlib import Path

from effects import bindings, old_sequences, self_delegate, timing
from hud import addressables_directory


class CombatAssetsTest(unittest.TestCase):
    def test_parameterless_carry_scripts_select_named_skeletal_clips(self):
        scripts = {11: {"type": 1, "rawParams": ""}}
        entries = [
            {"order": 1, "scriptId": 11, "casterMask":"C,*e3"},
            {"order": 2, "scriptId": 11, "casterMask":"C,*E3"},
        ]
        self.assertEqual([r["cast"] for r in bindings(entries, scripts, {50, 51})], ["AnimPickup", "AnimThrow"])
        self.assertEqual(bindings(entries, scripts, set()), [])

    def test_wrapper_inherits_only_an_immediate_guaranteed_self_cast(self):
        effect = {"actionId":1160, "targetMask":"C", "triggers":"I", "delay":0, "random":0,
                  "duration":0, "m_flags":64, "diceNum":24}
        levels = {1: {"effects":[effect], "criticalEffect":[]}, 2: {"effects":[dict(effect)], "criticalEffect":[]}}
        spell, animations = {"spellLevels":[1,2]}, {24:{"normal":[{"cast":"AnimAttaque2000"}]}}
        self.assertEqual(self_delegate(spell, levels, animations), [{"cast":"AnimAttaque2000", "atCaster":True}])
        levels[2]["effects"][0]["delay"] = 1
        self.assertEqual(self_delegate(spell, levels, animations), [])

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
