import hashlib
import json
import os
import tempfile
from typing import Dict, Iterable, Tuple


def _atomic_write(path: str, data: bytes, mode: int = 0o644) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    try:
        fd, tmp = tempfile.mkstemp(prefix=os.path.basename(path) + ".", dir=os.path.dirname(path))
        try:
            with os.fdopen(fd, "wb") as f:
                f.write(data)
            os.replace(tmp, path)
            os.chmod(path, mode)
        finally:
            try:
                if os.path.exists(tmp):
                    os.unlink(tmp)
            except OSError:
                pass
    except OSError:
        # Fallback for filesystems that don't support mkstemp in this dir
        with open(path + ".tmp", "wb") as f:
            f.write(data)
        os.replace(path + ".tmp", path)
        os.chmod(path, mode)


def sha256_bytes(b: bytes) -> str:
    return hashlib.sha256(b).hexdigest()


def write_json_atomic(path: str, obj: Dict, mode: int = 0o644) -> None:
    data = json.dumps(obj, indent=2, sort_keys=True).encode("utf-8")
    _atomic_write(path, data, mode)


def write_versions_json(path: str, artifact: str, host: str, versions: Iterable[Dict]) -> None:
    versions_list = list(versions)
    out = {
        "artifact": artifact,
        "host": host,
        "versions": versions_list,
    }
    write_json_atomic(path, out)


def write_docs_tree(root: str, files: Iterable[Tuple[str, bytes]]) -> Dict:
    # Write files and compute deterministic digest of the tree (sorted by path)
    paths = []
    for rel, content in files:
        target = os.path.join(root, rel)
        _atomic_write(target, content)
        paths.append((rel, content))
    paths.sort(key=lambda x: x[0])
    h = hashlib.sha256()
    for rel, content in paths:
        h.update(rel.encode("utf-8"))
        h.update(b"\x00")
        h.update(content)
        h.update(b"\x00")
    return {"docs_tree_sha256": h.hexdigest()}
