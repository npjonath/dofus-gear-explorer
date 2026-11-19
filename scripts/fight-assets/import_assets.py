"""Import combat art from an installed DOFUS 3 client into the local API's data directory."""
import argparse
import hashlib
import json
import os
import shutil
import sys
import tempfile
import types
from pathlib import Path


def configure_graphics_import():
    # UnityPy imports its audio converter even when only reading textures,
    # fonts and skeletal assets. Desktop builds omit the unused FMOD runtime.
    if getattr(sys, "frozen", False):
        audio = types.ModuleType("fmod_toolkit")

        def unsupported_audio(*args, **kwargs):
            raise NotImplementedError("The combat importer only extracts graphics.")

        audio.raw_to_wav = unsupported_audio
        sys.modules["fmod_toolkit"] = audio


def locate(source):
    candidates = (source, source / "DofusContent/Content", source / "Content",
                  source / "Dofus_Data/StreamingAssets/Content",
                  source / "Dofus.app/Contents/Resources/Data/StreamingAssets/Content",
                  source.parent / "DofusContent/Content")
    for candidate in candidates:
        if all((candidate / name).is_dir() for name in ("Map/Data", "Characters/Bones", "Characters/Skins")):
            return candidate
    raise ValueError("Cannot find the DOFUS 3 map and character catalogs in the selected installation.")


def import_pack(source, target):
    import UnityPy
    UnityPy.config.FALLBACK_UNITY_VERSION = "6000.0.44f1"
    from effects import export_effects
    from maps import export_maps
    from hud import addressables_directory, export_hud
    from actors import export_actors, prepare_equipment_catalog

    content = locate(source.resolve())
    target.mkdir(parents=True, exist_ok=True)
    # Both the extraction format and the source catalogs determine cache validity.
    identity = hashlib.sha256(b"dge-fight-assets-2")
    identity.update(prepare_equipment_catalog(target).encode())
    builtins = addressables_directory(content)
    # Content and the UI that ships in the executable have independent update catalogs.
    hud_catalog = builtins.parent / "catalog.hash"
    if hud_catalog.is_file():
        identity.update(hud_catalog.read_bytes())
    else:
        marker = (builtins / "figs_assets_2x.bundle").stat()
        identity.update(f"{marker.st_size}-{marker.st_mtime_ns}".encode())
    catalogs = sorted(content.rglob("catalog*.hash"))
    if not catalogs:
        raise ValueError("The DOFUS content catalogs are missing.")
    for catalog in catalogs:
        identity.update(catalog.relative_to(content).as_posix().encode())
        identity.update(catalog.read_bytes())
    name = "pack-" + identity.hexdigest()[:24]
    final = target / name
    if not (final / "complete.json").is_file():
        staging = Path(tempfile.mkdtemp(prefix=".import-", dir=target))
        try:
            for stage, export in (("maps", export_maps), ("hud", export_hud), ("actors", export_actors), ("effects",export_effects)):
                print(stage, flush=True)
                export(content, staging)
            (staging / "complete.json").write_text(json.dumps({"format": 1, "id": name}), encoding="utf-8")
            try:
                staging.rename(final)
            except OSError:
                if not (final / "complete.json").is_file():
                    raise
        finally:
            if staging.exists():
                shutil.rmtree(staging)
    with tempfile.NamedTemporaryFile(dir=target, mode="w", encoding="utf-8", delete=False) as pointer:
        pointer.write(name)
    os.replace(pointer.name, target / "current")
    # A patch can change gigabytes of textures. Keep the last generation for
    # in-flight reads; only prune older generations made by this importer.
    generations = sorted((entry for entry in target.glob("pack-*")
                          if not entry.is_symlink() and entry.is_dir() and (entry / "complete.json").is_file()),
                         key=lambda entry: entry.stat().st_mtime, reverse=True)
    older = [entry for entry in generations if entry != final]
    for entry in older[1:]:
        if (target / "current").read_text(encoding="utf-8").strip() != name:
            break
        try:
            metadata = json.loads((entry / "complete.json").read_text(encoding="utf-8"))
            if metadata.get("format") == 1 and metadata.get("id") == entry.name:
                shutil.rmtree(entry)
        except (OSError, ValueError):
            continue


if __name__ == "__main__":
    configure_graphics_import()
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    if sys.argv[1:] == ["--self-test"]:
        import actors, effects, maps, hud
        import UnityPy
        from PIL import Image
        from io import BytesIO
        stream = BytesIO()
        Image.new("RGBA", (2, 2), "red").save(stream, "WEBP", lossless=True)
        stream.seek(0)
        assert Image.open(stream).convert("RGBA").getpixel((0, 0)) == (255,0,0,255)
        print("Combat importer ready", flush=True)
        sys.exit(0)
    args = parser.parse_args()
    try:
        import_pack(args.source, args.output)
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
