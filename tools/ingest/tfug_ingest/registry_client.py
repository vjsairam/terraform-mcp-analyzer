import json
import urllib.request
import urllib.error
from typing import List, Optional
import os

from .types import ProviderRef, ModuleRef


def _get(url: str, timeout: int = 30) -> dict:
    req = urllib.request.Request(url, headers={"User-Agent": "TFUG-Ingest/1.0"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8"))


def discover_providers(host: str, seeds_path: Optional[str] = None) -> List[ProviderRef]:
    # Prefer protocol; fallback to seeds file lines like: provider:hashicorp/aws
    refs: List[ProviderRef] = []
    if seeds_path and os.path.exists(seeds_path):
        with open(seeds_path, "r", encoding="utf-8") as f:
            for line in f:
                s = line.strip()
                if not s or s.startswith("#"):
                    continue
                if s.startswith("provider:"):
                    body = s.split(":", 1)[1]
                    if "/" in body:
                        ns, name = body.split("/")
                        refs.append(ProviderRef(host=host, namespace=ns, name=name, source_id=s))
    return refs


def discover_modules(host: str, seeds_path: Optional[str] = None) -> List[ModuleRef]:
    refs: List[ModuleRef] = []
    if seeds_path and os.path.exists(seeds_path):
        with open(seeds_path, "r", encoding="utf-8") as f:
            for line in f:
                s = line.strip()
                if not s or s.startswith("#"):
                    continue
                if s.startswith("module:"):
                    body = s.split(":", 1)[1]
                    parts = body.split("/")
                    if len(parts) == 3:
                        ns, name, prov = parts
                        refs.append(ModuleRef(host=host, namespace=ns, name=name, provider=prov, source_id=s))
    return refs


def list_provider_versions(ref: ProviderRef) -> List[dict]:
    # Provider versions endpoint: /v1/providers/{namespace}/{name}/versions
    url = f"https://{ref.host}/v1/providers/{ref.namespace}/{ref.name}/versions"
    try:
        data = _get(url)
        versions = data.get("versions", [])
        out = []
        for v in versions:
            out.append({
                "version": v.get("version"),
                "yanked": bool(v.get("yanked", False)),
            })
        return out
    except Exception:
        return []


def list_module_versions(ref: ModuleRef) -> List[dict]:
    # Module versions endpoint: /v1/modules/{namespace}/{name}/{provider}/versions
    url = f"https://{ref.host}/v1/modules/{ref.namespace}/{ref.name}/{ref.provider}/versions"
    try:
        data = _get(url)
        modules = data.get("modules", [])
        if not modules:
            return []
        versions = modules[0].get("versions", [])
        out = []
        for v in versions:
            out.append({"version": v.get("version"), "yanked": False})
        return out
    except Exception:
        return []
