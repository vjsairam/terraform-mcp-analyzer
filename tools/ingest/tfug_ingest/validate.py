import json
import os
from typing import Tuple


def validate_root(root: str) -> Tuple[int, int, int]:
    """Validate artifacts tree. Returns (errors, providers_checked, modules_checked)."""
    errors = 0
    provs = 0
    mods = 0
    prov_root = os.path.join(root, "providers")
    mod_root = os.path.join(root, "modules")

    def _validate_leaf(leaf_dir: str) -> int:
        errs = 0
        verfile = os.path.join(leaf_dir, "versions.json")
        if not os.path.isfile(verfile):
            return 1
        try:
            with open(verfile, "r", encoding="utf-8") as f:
                vdata = json.load(f)
        except Exception:
            return 1
        # For each version dir present, ensure meta.json exists and has minimal fields
        for d in os.listdir(leaf_dir):
            if not d.startswith("v"):
                continue
            vdir = os.path.join(leaf_dir, d)
            if not os.path.isdir(vdir):
                continue
            meta = os.path.join(vdir, "meta.json")
            if not os.path.isfile(meta):
                errs += 1
                continue
            try:
                with open(meta, "r", encoding="utf-8") as f:
                    m = json.load(f)
                # Minimal checks
                if not m.get("artifact") or not m.get("version"):
                    errs += 1
                # docs present (docs/*.md) or origin/docs.html
                docs_dir = os.path.join(vdir, "docs")
                origin_html = os.path.join(vdir, "origin", "docs.html")
                has_docs = os.path.isdir(docs_dir) and any(
                    name.endswith(".md") for name in os.listdir(docs_dir)
                )
                has_html = os.path.isfile(origin_html)
                if not (has_docs or has_html):
                    errs += 1
            except Exception:
                errs += 1
        return errs

    if os.path.isdir(prov_root):
        for host in os.listdir(prov_root):
            hdir = os.path.join(prov_root, host)
            if not os.path.isdir(hdir):
                continue
            for leaf in os.listdir(hdir):
                ldir = os.path.join(hdir, leaf)
                if not os.path.isdir(ldir):
                    continue
                provs += 1
                errors += _validate_leaf(ldir)
    if os.path.isdir(mod_root):
        for host in os.listdir(mod_root):
            hdir = os.path.join(mod_root, host)
            if not os.path.isdir(hdir):
                continue
            for leaf in os.listdir(hdir):
                ldir = os.path.join(hdir, leaf)
                if not os.path.isdir(ldir):
                    continue
                mods += 1
                errors += _validate_leaf(ldir)
    return errors, provs, mods

