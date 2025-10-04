import argparse
import datetime as dt
import json
import os
import sys
from typing import List

from .types import ProviderRef, ModuleRef, VersionMeta
from . import registry_client as rc
from . import writer
from . import registry_docs
from . import repo_docs
from .convert import html_to_markdown
from .validate import validate_root
from types import SimpleNamespace
import time
import random
from concurrent.futures import ThreadPoolExecutor, as_completed
from urllib.parse import urlparse


def _now_iso() -> str:
    return dt.datetime.utcnow().replace(microsecond=0).isoformat() + "Z"


def cmd_discover(args: argparse.Namespace) -> int:
    provs: List[ProviderRef] = []
    mods: List[ModuleRef] = []
    if args.providers:
        provs = rc.discover_providers(args.host, getattr(args, "seeds", None))
    if args.modules:
        mods = rc.discover_modules(args.host, getattr(args, "seeds", None))
    # Snapshot emit
    os.makedirs(args.out, exist_ok=True)
    snap = {
        "generated_at": _now_iso(),
        "providers": [p.source_id for p in provs],
        "modules": [m.source_id for m in mods],
    }
    out_path = os.path.join(args.out, f"discovery-{dt.datetime.utcnow():%Y%m%d}.json")
    writer.write_json_atomic(out_path, snap)
    print(out_path)
    return 0


def _artifact_paths(base_out: str, host: str, kind: str, parts: List[str]) -> str:
    # artifacts/providers/<host>/<namespace>.<name>/ or modules/<host>/<namespace>.<name>.<provider>/
    if kind == "provider":
        ns, name = parts
        leaf = f"{ns}.{name}"
        return os.path.join(base_out, "providers", host, leaf)
    else:
        ns, name, prov = parts
        leaf = f"{ns}.{name}.{prov}"
        return os.path.join(base_out, "modules", host, leaf)


def _parse_artifact(s: str, host: str):
    # provider:hashicorp/aws  |  module:terraform-aws-modules/iam/aws
    if s.startswith("provider:"):
        body = s.split(":", 1)[1]
        ns, name = body.split("/")
        ref = ProviderRef(host=host, namespace=ns, name=name, source_id=s)
        return "provider", ref
    if s.startswith("module:"):
        body = s.split(":", 1)[1]
        ns, name, prov = body.split("/")
        ref = ModuleRef(host=host, namespace=ns, name=name, provider=prov, source_id=s)
        return "module", ref
    raise SystemExit("invalid --artifact; expected provider:ns/name or module:ns/name/provider")


def cmd_fetch(args: argparse.Namespace) -> int:
    kind, ref = _parse_artifact(args.artifact, args.host)
    out_root = args.out
    if kind == "provider":
        vdicts = rc.list_provider_versions(ref)
        versions = _sort_versions_desc([v.get("version") for v in vdicts if v.get("version")])
        base = _artifact_paths(out_root, args.host, kind, [ref.namespace, ref.name])
        writer.write_versions_json(os.path.join(base, "versions.json"), ref.source_id, args.host, [{"version": v, "yanked": False} for v in versions])
        # choose versions
        selected: List[str] = []
        if getattr(args, 'all', False):
            selected = list(versions)
        elif args.latest and versions:
            selected = [versions[0]]
        if getattr(args, "prev_minor", 0) > 0 and versions:
            selected = _latest_prev_minors(versions, args.prev_minor)
        if args.deep > 0 and versions:
            selected = versions[: args.deep]
        for ver in selected:
            vdir = os.path.join(base, f"v{ver}")
            meta = {
                "artifact": ref.source_id,
                "version": ver,
                "host": args.host,
                "fetched": {},
                "digests": {},
            }
            # Attempt repo docs first
            prior_meta = _load_meta(vdir)
            if prior_meta:
                # carry forward known fields if present
                for k in ("repo_url",):
                    if k in prior_meta:
                        meta[k] = prior_meta[k]
            repo_url = getattr(args, "repo_url", "") or _infer_repo_url("provider", ref)
            files = []
            if repo_url:
                files = repo_docs.fetch_tree(repo_url, f"v{ver}")
                if files:
                    meta.setdefault("repo_url", repo_url)
            if files:
                dig = writer.write_docs_tree(os.path.join(vdir, "docs"), files)
                meta["digests"].update(dig)
                meta.setdefault("fetched", {}).update({"docs_repo": {"ref": f"v{ver}", "fetched_at": _now_iso()}})
                meta["docs_format"] = "md"
            else:
                # Fallback: registry HTML (provider docs)
                if getattr(args, "html_fallback", True):
                    docs_url = args.docs_url or f"https://{args.host}/providers/{ref.namespace}/{ref.name}/{ver}/docs"
                    etag = _get_path(prior_meta or {}, ["fetched", "docs_html", "etag"]) if getattr(args, "respect_etag", True) else None
                    lastm = _get_path(prior_meta or {}, ["fetched", "docs_html", "last_modified"]) if getattr(args, "respect_etag", True) else None
                    resp = registry_docs.fetch_html(docs_url, etag, lastm)
                    if resp.get("status") == 304:
                        # Not modified; preserve prior meta
                        writer.write_json_atomic(os.path.join(vdir, "meta.json"), prior_meta or meta)
                        continue
                    if resp.get("status", 200) == 200:
                        # provenance
                        origin = os.path.join(vdir, "origin")
                        os.makedirs(origin, exist_ok=True)
                        with open(os.path.join(origin, "docs.html"), "wb") as f:
                            f.write(resp["html"]) 
                        parts = html_to_markdown(resp["html"]).items()
                        dig = writer.write_docs_tree(os.path.join(vdir, "docs"), parts)
                        meta["digests"].update(dig)
                        meta.setdefault("fetched", {}).update({
                            "docs_html": {"etag": resp.get("etag"), "last_modified": resp.get("last_modified"), "fetched_at": _now_iso()},
                        })
                        meta["docs_format"] = "md"
            writer.write_json_atomic(os.path.join(vdir, "meta.json"), meta)
        return 0
    else:
        vdicts = rc.list_module_versions(ref)
        versions = _sort_versions_desc([v.get("version") for v in vdicts if v.get("version")])
        base = _artifact_paths(out_root, args.host, kind, [ref.namespace, ref.name, ref.provider])
        writer.write_versions_json(os.path.join(base, "versions.json"), ref.source_id, args.host, [{"version": v, "yanked": False} for v in versions])
        selected = []
        if getattr(args, 'all', False):
            selected = list(versions)
        elif args.latest and versions:
            selected = [versions[0]]
        if getattr(args, "prev_minor", 0) > 0 and versions:
            selected = _latest_prev_minors(versions, args.prev_minor)
        if args.deep > 0 and versions:
            selected = versions[: args.deep]
        for ver in selected:
            vdir = os.path.join(base, f"v{ver}")
            meta = {
                "artifact": ref.source_id,
                "version": ver,
                "host": args.host,
                "fetched": {},
                "digests": {},
            }
            # Attempt repo docs
            prior_meta = _load_meta(vdir)
            if prior_meta:
                for k in ("repo_url",):
                    if k in prior_meta:
                        meta[k] = prior_meta[k]
            repo_url = getattr(args, "repo_url", "") or _infer_repo_url("module", ref)
            files = []
            if repo_url:
                files = repo_docs.fetch_tree(repo_url, f"v{ver}")
                if files:
                    meta.setdefault("repo_url", repo_url)
                    dig = writer.write_docs_tree(os.path.join(vdir, "docs"), files)
                    meta["digests"].update(dig)
                    meta.setdefault("fetched", {}).update({"docs_repo": {"ref": f"v{ver}", "fetched_at": _now_iso()}})
                    meta["docs_format"] = "md"
            # Fallback: registry HTML (module page) when repo docs not found
            if not files and getattr(args, "html_fallback", True):
                docs_url = args.docs_url or f"https://{args.host}/modules/{ref.namespace}/{ref.name}/{ref.provider}/{ver}"
                etag = _get_path(prior_meta or {}, ["fetched", "docs_html", "etag"]) if getattr(args, "respect_etag", True) else None
                lastm = _get_path(prior_meta or {}, ["fetched", "docs_html", "last_modified"]) if getattr(args, "respect_etag", True) else None
                resp = registry_docs.fetch_html(docs_url, etag, lastm)
                if resp.get("status") == 304:
                    writer.write_json_atomic(os.path.join(vdir, "meta.json"), prior_meta or meta)
                elif resp.get("status", 200) == 200:
                    origin = os.path.join(vdir, "origin")
                    os.makedirs(origin, exist_ok=True)
                    with open(os.path.join(origin, "docs.html"), "wb") as f:
                        f.write(resp["html"])
                    parts = html_to_markdown(resp["html"]).items()
                    dig = writer.write_docs_tree(os.path.join(vdir, "docs"), parts)
                    meta["digests"].update(dig)
                    meta.setdefault("fetched", {}).update({
                        "docs_html": {"etag": resp.get("etag"), "last_modified": resp.get("last_modified"), "fetched_at": _now_iso()},
                    })
                    meta["docs_format"] = "md"
            writer.write_json_atomic(os.path.join(vdir, "meta.json"), meta)
        return 0


def _infer_repo_url(kind: str, ref) -> str:
    # Minimal known mappings to bootstrap repo-docs; extend as needed
    if kind == "provider":
        if getattr(ref, "namespace", "") == "hashicorp":
            name = getattr(ref, "name", "")
            if name == "aws":
                return "https://github.com/hashicorp/terraform-provider-aws"
            if name == "azurerm":
                return "https://github.com/hashicorp/terraform-provider-azurerm"
            if name == "google":
                return "https://github.com/hashicorp/terraform-provider-google"
            if name == "kubernetes":
                return "https://github.com/hashicorp/terraform-provider-kubernetes"
    elif kind == "module":
        if getattr(ref, "namespace", "") == "terraform-aws-modules":
            mname = getattr(ref, "name", "")
            if mname == "iam":
                return "https://github.com/terraform-aws-modules/terraform-aws-iam"
            if mname == "vpc":
                return "https://github.com/terraform-aws-modules/terraform-aws-vpc"
            if mname == "eks":
                return "https://github.com/terraform-aws-modules/terraform-aws-eks"
            if mname in ("s3-bucket", "s3_bucket"):
                return "https://github.com/terraform-aws-modules/terraform-aws-s3-bucket"
    return ""


def _parse_semver(ver: str):
    try:
        s = ver.lstrip("v").split("-")[0].split("+")[0]
        parts = s.split(".")
        major = int(parts[0]) if len(parts) > 0 else 0
        minor = int(parts[1]) if len(parts) > 1 else 0
        patch = int(parts[2]) if len(parts) > 2 else 0
        return (major, minor, patch)
    except Exception:
        return (0, 0, 0)


def _sort_versions_desc(versions: List[str]) -> List[str]:
    return sorted(versions, key=lambda v: _parse_semver(v), reverse=True)


def _latest_prev_minors(versions: List[str], n: int) -> List[str]:
    seen = set()
    out: List[str] = []
    for v in _sort_versions_desc(versions):
        major, minor, _ = _parse_semver(v)
        key = (major, minor)
        if key not in seen:
            out.append(v)
            seen.add(key)
            if len(seen) >= n + 1:  # latest plus N previous minors
                break
    return out


def _load_meta(vdir: str):
    mpath = os.path.join(vdir, "meta.json")
    if not os.path.isfile(mpath):
        return None
    try:
        with open(mpath, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return None


def _get_path(obj, keys):
    cur = obj
    for k in keys:
        if not isinstance(cur, dict) or k not in cur:
            return None
        cur = cur[k]
    return cur


def cmd_snapshot(args: argparse.Namespace) -> int:
    # Walk artifacts tree and write a compact manifest snapshot
    providers = []
    modules = []
    docs_md = 0
    docs_html = 0
    root = args.root
    prov_root = os.path.join(root, "providers")
    mod_root = os.path.join(root, "modules")
    if os.path.isdir(prov_root):
        for host in sorted(os.listdir(prov_root)):
            hdir = os.path.join(prov_root, host)
            if not os.path.isdir(hdir):
                continue
            for leaf in sorted(os.listdir(hdir)):
                ldir = os.path.join(hdir, leaf)
                if not os.path.isdir(ldir):
                    continue
                versions = [d[1:] for d in sorted(os.listdir(ldir)) if d.startswith("v")]
                # Count docs format
                for d in sorted(os.listdir(ldir)):
                    if not d.startswith("v"):
                        continue
                    vdir = os.path.join(ldir, d)
                    docs_dir = os.path.join(vdir, "docs")
                    if os.path.isdir(docs_dir) and any(name.endswith(".md") for name in os.listdir(docs_dir)):
                        docs_md += 1
                    origin_html = os.path.join(vdir, "origin", "docs.html")
                    if os.path.isfile(origin_html):
                        docs_html += 1
                providers.append({"id": f"provider:{leaf.replace('.', '/')}", "host": host, "versions": versions})
    if os.path.isdir(mod_root):
        for host in sorted(os.listdir(mod_root)):
            hdir = os.path.join(mod_root, host)
            if not os.path.isdir(hdir):
                continue
            for leaf in sorted(os.listdir(hdir)):
                ldir = os.path.join(hdir, leaf)
                if not os.path.isdir(ldir):
                    continue
                versions = [d[1:] for d in sorted(os.listdir(ldir)) if d.startswith("v")]
                for d in sorted(os.listdir(ldir)):
                    if not d.startswith("v"):
                        continue
                    vdir = os.path.join(ldir, d)
                    docs_dir = os.path.join(vdir, "docs")
                    if os.path.isdir(docs_dir) and any(name.endswith(".md") for name in os.listdir(docs_dir)):
                        docs_md += 1
                    origin_html = os.path.join(vdir, "origin", "docs.html")
                    if os.path.isfile(origin_html):
                        docs_html += 1
                modules.append({"id": f"module:{leaf.replace('.', '/')}".replace("/", "/", 2), "host": host, "versions": versions})
    snap = {"generated_at": _now_iso(), "providers": providers, "modules": modules, "metrics": {"docs_md_count": docs_md, "docs_html_count": docs_html}}
    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    writer.write_json_atomic(args.out, snap)
    print(args.out)
    return 0


def _cmd_validate(args: argparse.Namespace) -> int:
    errs, provs, mods = validate_root(args.root)
    print(json.dumps({"providers": provs, "modules": mods, "errors": errs}))
    return 1 if errs else 0


def _read_seeds(path: str) -> List[str]:
    seeds: List[str] = []
    try:
        with open(path, "r", encoding="utf-8") as f:
            for line in f:
                s = line.strip()
                if not s or s.startswith("#"):
                    continue
                if s.startswith("provider:") or s.startswith("module:"):
                    seeds.append(s)
                    continue
                # Accept registry URLs and convert to artifact ids
                if s.startswith("http://") or s.startswith("https://"):
                    u = urlparse(s)
                    parts = [p for p in u.path.split("/") if p]
                    if len(parts) >= 3 and parts[0] == "providers":
                        ns, name = parts[1], parts[2]
                        seeds.append(f"provider:{ns}/{name}")
                        continue
                    if len(parts) >= 4 and parts[0] == "modules":
                        ns, name, prov = parts[1], parts[2], parts[3]
                        seeds.append(f"module:{ns}/{name}/{prov}")
                        continue
    except FileNotFoundError:
        pass
    return seeds


def _fetch_artifact_seed(host: str, seed: str, out: str, latest: bool, prev_minor: int, deep: int, respect_etag: bool, html_fallback: bool, all_versions: bool=False) -> dict:
    args = SimpleNamespace(
        host=host,
        artifact=seed,
        latest=latest,
        all=all_versions,
        prev_minor=prev_minor,
        deep=deep,
        docs_url="",
        repo_url="",
        respect_etag=respect_etag,
        html_fallback=html_fallback,
        out=out,
    )
    try:
        cmd_fetch(args)
        return {"seed": seed, "ok": True}
    except SystemExit as e:
        return {"seed": seed, "ok": False, "error": f"exit {e.code}"}
    except Exception as e:
        return {"seed": seed, "ok": False, "error": str(e)}


def cmd_batch(args: argparse.Namespace) -> int:
    seeds = _read_seeds(args.seeds)
    if args.limit and len(seeds) > args.limit:
        seeds = seeds[: args.limit]
    if not seeds:
        print(json.dumps({"processed": 0, "errors": 0}))
        return 0
    os.makedirs(args.out, exist_ok=True)
    results = []
    # Polite, small pool
    with ThreadPoolExecutor(max_workers=max(1, int(args.concurrency))) as ex:
        futs = []
        for s in seeds:
            # Small jitter to avoid burst; additional host-level backoff is inside fetchers
            time.sleep(0.1)
            futs.append(ex.submit(_fetch_artifact_seed, args.host, s, args.out, args.latest, args.prev_minor, args.deep, args.respect_etag, args.html_fallback, args.all))
        for fut in as_completed(futs):
            results.append(fut.result())
    errors = [r for r in results if not r.get("ok")]
    # Write a simple run summary and errors.ndjson
    summary = {"processed": len(results), "errors": len(errors)}
    writer.write_json_atomic(os.path.join(args.out, "batch_summary.json"), summary)
    if errors:
        with open(os.path.join(args.out, "errors.ndjson"), "ab") as f:
            for e in errors:
                line = (json.dumps(e) + "\n").encode("utf-8")
                f.write(line)
    # Validate and snapshot
    _ = _cmd_validate(SimpleNamespace(root=args.out))
    snap_path = os.path.join(args.out, f"snapshot-{dt.datetime.utcnow():%Y%m%d}.json")
    _ = cmd_snapshot(SimpleNamespace(root=args.out, out=snap_path))
    print(json.dumps({"summary": summary, "snapshot": snap_path}))
    return 0


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="tfug-ingest", description="TFUG scraper (protocol-first, deterministic)")
    sub = p.add_subparsers(dest="cmd", required=True)

    d = sub.add_parser("discover")
    d.add_argument("--host", default="registry.terraform.io")
    d.add_argument("--providers", action="store_true")
    d.add_argument("--modules", action="store_true")
    d.add_argument("--out", default="artifacts")
    d.add_argument("--seeds", default="tools/registry-scrape/seeds.txt")
    d.set_defaults(fn=cmd_discover)

    f = sub.add_parser("fetch")
    f.add_argument("--host", default="registry.terraform.io")
    f.add_argument("--artifact", required=True)
    f.add_argument("--latest", action="store_true")
    f.add_argument("--prev-minor", type=int, default=0)
    f.add_argument("--deep", type=int, default=0)
    f.add_argument("--docs-url", default="")
    f.add_argument("--repo-url", default="")
    f.add_argument("--respect-etag", action="store_true", default=True)
    f.add_argument("--html-fallback", action="store_true", default=True)
    f.add_argument("--out", default="artifacts")
    f.set_defaults(fn=cmd_fetch)

    s = sub.add_parser("snapshot")
    s.add_argument("--root", default="artifacts")
    s.add_argument("--out", required=True)
    s.set_defaults(fn=cmd_snapshot)

    v = sub.add_parser("validate")
    v.add_argument("--root", default="artifacts")
    v.set_defaults(fn=lambda a: _cmd_validate(a))

    b = sub.add_parser("batch")
    b.add_argument("--host", default="registry.terraform.io")
    b.add_argument("--seeds", default="tools/registry-scrape/seeds.txt")
    b.add_argument("--out", default="artifacts")
    b.add_argument("--latest", action="store_true")
    b.add_argument("--prev-minor", type=int, default=0)
    b.add_argument("--deep", type=int, default=0)
    b.add_argument("--respect-etag", action="store_true", default=True)
    b.add_argument("--html-fallback", action="store_true", default=True)
    b.add_argument("--concurrency", type=int, default=3)
    b.add_argument("--all", action="store_true", help="fetch all discovered versions for each artifact")
    b.add_argument("--limit", type=int, default=0, help="limit number of seeds processed (0 = all)")
    b.set_defaults(fn=lambda a: cmd_batch(a))

    args = p.parse_args(argv)
    return args.fn(args)


if __name__ == "__main__":
    sys.exit(main())
