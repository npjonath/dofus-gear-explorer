import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from PIL import Image
from UnityPy.enums import BuildTarget, TextureFormat
from UnityPy.export.Texture2DConverter import parse_image_data

import maps


def tile(number, moves=True, sees=True, red=False, blue=False):
    return {"cellNumber": number, "mov": moves, "los": sees, "red": red, "blue": blue}


def element(gfx, x=0, y=0, tint=0xFF808080):
    return {
        "gfxId": gfx, "position": {"x": x, "y": y}, "scale": {"x": 1, "y": 1},
        "rotation": 0, "color": {"value": tint},
    }


class MapImportTest(unittest.TestCase):
    def test_staggered_rows_become_combat_neighbors_at_map_edge(self):
        # Cell 0 of the original map. The up-left corner of the enclosing
        # combat rectangle is outside the game's playable diamond.
        cells = [tile(0, red=True), tile(1), tile(14, False, True),
                 tile(15, blue=True), tile(28), tile(29, False, False)]
        grid = maps._grid({"id": 1, "cellsData": cells}, Image.new("RGB", (200, 200)), -700, -650)
        self.assertEqual((grid["width"], grid["height"]), (3, 3))
        self.assertEqual(grid["red"], [3])
        self.assertEqual(grid["blue"], [2])
        self.assertEqual(grid["walls"], [0, 4, 5, 6, 8])
        self.assertEqual(grid["sightWalls"], [0, 5, 6, 8])
        x = grid["originX"] - grid["halfCellWidth"]
        y = grid["originY"] + grid["halfCellHeight"]
        self.assertEqual((x, y), (119.5, 166.25))

    def test_map_tint_and_alpha_apply_before_layer_compositing(self):
        image = Image.new("RGBA", (2, 1))
        image.putdata([(20, 90, 40, 128), (100, 40, 30, 255)])
        obj = element(1, tint=0x804080FF)
        obj["scale"]["x"] = -1
        stamp, _, _ = maps._stamp(obj, {1: image})
        self.assertEqual(stamp.getpixel((0, 0)), (50, 40, 60, 128))
        self.assertEqual(stamp.getpixel((1, 0)), (10, 90, 80, 64))
        canvas = Image.new("RGB", (2, 1), (200, 140, 88))
        maps._composite(canvas, stamp, 0, 0, {"sourceFactor": 2, "destinationFactor": 10})
        self.assertEqual(canvas.getpixel((0, 0)), (119, 80, 54))

    def test_compressed_desktop_textures(self):
        # Independently encoded constant-colour 4×4 blocks, with no game artwork.
        blocks = (
            (TextureFormat.DXT5, "ed000000000000004592459255555555", (148, 73, 41, 237)),
            (TextureFormat.BC7, "40a59244a250ee770000000000000000", (148, 72, 40, 238)),
            (TextureFormat.ASTC_RGBA_4x4, "fcfdffffffffffff949448482828eded", (148, 72, 40, 237)),
        )
        for fmt, block, expected in blocks:
            for platform in (BuildTarget.StandaloneWindows64, BuildTarget.StandaloneLinux64, BuildTarget.StandaloneOSX):
                with self.subTest(format=fmt.name, platform=platform.name):
                    image = parse_image_data(bytes.fromhex(block), 4, 4, fmt, (6000, 3, 16, 1), platform)
                    self.assertEqual(image.getpixel((0, 0)), expected)

    def test_one_room_with_mixed_case_addresses_and_only_2x_textures(self):
        data = {
            "id": maps.VILLAGE, "backgroundColor": {"value": 0xFF000000},
            "cellsData": [tile(0, red=True), tile(14, blue=True)],
            "backgroundMapElements": [{"mapElements": [{"rid": 1}], "materialIndex": 0}],
            "middlegroundMapElements": [], "foregroundMapElements": [],
            "backgroundMaterialData": {"shaderData": [{"rid": 2}]},
            "middlegroundMaterialData": {"shaderData": []}, "foregroundMaterialData": {"shaderData": []},
            "references": {"RefIds": [
                {"rid": 1, "data": element(123, -580.5, 483.75)},
                {"rid": 2, "data": {"shaderParameters": []}},
            ]},
        }
        map_asset = SimpleNamespace(deref_parse_as_dict=lambda: data)
        obj = SimpleNamespace(image=Image.new("RGBA", (24, 20), (20, 120, 200, 255)))
        tex_asset = SimpleNamespace(deref_parse_as_object=lambda: obj,
                                    deref=lambda: SimpleNamespace(type=SimpleNamespace(name="Texture2D")))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            content = root / "Dofus_Data" / "StreamingAssets" / "Content"
            bundle = content / "Map/Data/MAPDATA_ASSETS_WORLD_378.BUNDLE"
            texture = content / "Map/Textures/2X/mapgfx_2x_0_assets_all.bundle"
            for file in (bundle, texture):
                file.parent.mkdir(parents=True, exist_ok=True)
                file.touch()
            (root / "Dofus_Data/globalgamemanagers").write_bytes(b"\x00" * 48 + b"6000.4.12f3\x00")
            self.assertEqual(maps._client_unity_version(content), "6000.4.12f3")
            def load(path):
                if Path(path).samefile(bundle):
                    return SimpleNamespace(container={r"MAPS\MAP_99222029.ASSET": map_asset})
                self.assertTrue(Path(path).samefile(texture))
                return SimpleNamespace(container={r"GFX\123.PNG": tex_asset})

            with patch.object(maps.UnityPy, "load", side_effect=load):
                maps.export_maps(content, root / "export")
            written = root / "export/maps"
            self.assertEqual({p.name for p in written.iterdir()}, {"99222029.json", "99222029.webp"})
            spec = json.loads((written / "99222029.json").read_text())
            self.assertEqual((spec["width"], spec["height"], spec["red"], spec["blue"]), (2, 1, [0], [1]))
            with Image.open(written / "99222029.webp") as image:
                self.assertEqual(image.size, (12, 10))
                self.assertEqual(image.getpixel((6, 5)), (20, 120, 200))

    def test_incomplete_installation_identifies_missing_resources(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaisesRegex(FileNotFoundError, "neither training room"):
                maps.export_maps(root, root / "output")
            with self.assertRaisesRegex(FileNotFoundError, r"group 70000.*71291"):
                maps._read_textures(root, {71291})


if __name__ == "__main__":
    unittest.main()
