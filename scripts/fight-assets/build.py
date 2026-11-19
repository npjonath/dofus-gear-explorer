"""Build a self-contained importer for the host that will run it."""
import os
import platform
import shutil
from importlib.util import find_spec
import sys
import tempfile
from pathlib import Path
from importlib.metadata import distributions

from PyInstaller.__main__ import run

here = Path(__file__).resolve().parent
os_name = {"darwin": "mac", "win32": "win", "linux": "linux"}[sys.platform]
arch = {"aarch64": "arm64", "arm64": "arm64", "amd64": "x64", "x86_64": "x64"}[platform.machine().lower()]
dest = here.parents[1] / "apps" / "desktop" / "fight-import" / f"{os_name}-{arch}"
dest.parent.mkdir(parents=True, exist_ok=True)

with tempfile.TemporaryDirectory() as folder:
    os.environ.setdefault("PYINSTALLER_CONFIG_DIR", str(Path(folder) / "cache"))
    archspec_root = Path(find_spec("archspec").origin).parent
    run(["--clean", "--noconfirm", "--onedir", "--name", "fight-import",
         "--workpath", str(Path(folder)/"work"), "--specpath", folder, "--distpath", str(Path(folder)/"dist"),
         "--add-data", f"{archspec_root / 'cpu' / 'schema.py'}:archspec/cpu",
         "--collect-data", "UnityPy", "--collect-data", "archspec", "--collect-data", "tpk_ar",
         "--collect-all", "texture2ddecoder", "--collect-all", "etcpak",
         "--collect-all", "astc_encoder", "--collect-all", "fmod_toolkit",
         "--collect-all", "brotli", "--copy-metadata", "UnityPy", str(here/"import_assets.py")])
    with tempfile.TemporaryDirectory(dir=dest.parent) as replacement:
        stage = Path(replacement) / "new"
        shutil.copytree(Path(folder) / "dist" / "fight-import", stage, symlinks=True)
        backup = Path(replacement) / "previous"
        if dest.exists():
            dest.rename(backup)
        try:
            stage.rename(dest)
        except OSError:
            if backup.exists():
                backup.rename(dest)
            raise

notices = []
python_license = Path(os.__file__).parent / "LICENSE.txt"
if python_license.is_file():
    notices.append(python_license.read_text(encoding="utf-8"))
for dist in sorted(distributions(), key=lambda d: d.metadata["Name"].lower()):
    for file in dist.files or ():
        if any(word in file.name.lower() for word in ("license", "licence", "copying", "notice")):
            try:
                text = Path(dist.locate_file(file)).read_text(encoding="utf-8")
            except (UnicodeDecodeError, OSError):
                continue
            notices.append(f'{dist.metadata["Name"]} {dist.version} — {file.name}\n{text}')
(dest/"LICENSES.txt").write_text("\n\n".join(notices), encoding="utf-8")
