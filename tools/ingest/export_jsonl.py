#!/usr/bin/env python3
"""
Export ingest corpus (providers/modules with all versions) to JSONL files
compatible with tools/terraformdocs-normalize.

Emits:
- _to_review/terraform_db_export/providers.jsonl
- _to_review/terraform_db_export/modules.jsonl

Heuristics:
- For each version directory vX.Y.Z, prefer Markdown under docs/ (README.md,
  index.md, or first .md). Fallback to origin/docs.html or first .html under docs/.
- Title derived from first Markdown "# " header or synthesized.
- URL reconstructed to registry URL for traceability.
"""
import argparse
import os
import json
from datetime import datetime, timezone


def now_iso():
    return datetime.now(timezone.utc).isoformat()


def read_text(path):
    try:
        with open(path, 'r', encoding='utf-8') as f:
            return f.read()
    except Exception:
        return ''


def first_header(md: str) -> str:
    for line in md.splitlines():
        s = line.strip()
        if s.startswith('#'):
            return s.lstrip('#').strip()
    return ''


def pick_content(vdir: str):
    # returns (content, content_type, title)
    docs = os.path.join(vdir, 'docs')
    # Priority: README.md, index.md, any .md
    md_candidates = []
    if os.path.isdir(docs):
        for root, _dirs, files in os.walk(docs):
            for fn in files:
                if fn.lower() in ('readme.md', 'index.md'):
                    p = os.path.join(root, fn)
                    md = read_text(p)
                    return md, 'md', first_header(md)
                if fn.lower().endswith('.md'):
                    md_candidates.append(os.path.join(root, fn))
    if md_candidates:
        md_candidates.sort()
        md = read_text(md_candidates[0])
        return md, 'md', first_header(md)
    # Fallback: origin/docs.html
    origin = os.path.join(vdir, 'origin', 'docs.html')
    if os.path.isfile(origin):
        html = read_text(origin)
        return html, 'html', ''
    # Fallback: any .html under docs
    if os.path.isdir(docs):
        html_candidates = []
        for root, _dirs, files in os.walk(docs):
            for fn in files:
                if fn.lower().endswith('.html'):
                    html_candidates.append(os.path.join(root, fn))
        if html_candidates:
            html_candidates.sort()
            html = read_text(html_candidates[0])
            return html, 'html', ''
    return '', 'md', ''


def export_providers(root: str, out_path: str):
    base = os.path.join(root, 'providers')
    count = 0
    if not os.path.isdir(base):
        return 0
    with open(out_path, 'w', encoding='utf-8') as out:
        for host in sorted(os.listdir(base)):
            hdir = os.path.join(base, host)
            if not os.path.isdir(hdir):
                continue
            for leaf in sorted(os.listdir(hdir)):
                # leaf like namespace.name
                if '.' not in leaf:
                    continue
                ns, name = leaf.split('.', 1)
                ldir = os.path.join(hdir, leaf)
                if not os.path.isdir(ldir):
                    continue
                for d in sorted(os.listdir(ldir)):
                    if not d.startswith('v'):
                        continue
                    ver = d[1:]
                    vdir = os.path.join(ldir, d)
                    content, ctype, title = pick_content(vdir)
                    if not content:
                        continue
                    url = f"https://{host}/providers/{ns}/{name}/{ver}/docs"
                    rec = {
                        'type': 'provider',
                        'namespace': ns,
                        'name': name,
                        'version': ver,
                        'url': url,
                        'title': title or f"{name} provider",
                        'content': content,
                        'content_type': ctype,
                        'scraped_at': now_iso(),
                    }
                    out.write(json.dumps(rec, ensure_ascii=False) + "\n")
                    count += 1
    return count


def export_modules(root: str, out_path: str):
    base = os.path.join(root, 'modules')
    count = 0
    if not os.path.isdir(base):
        return 0
    with open(out_path, 'w', encoding='utf-8') as out:
        for host in sorted(os.listdir(base)):
            hdir = os.path.join(base, host)
            if not os.path.isdir(hdir):
                continue
            for leaf in sorted(os.listdir(hdir)):
                # leaf like namespace.name.provider
                parts = leaf.split('.')
                if len(parts) < 3:
                    continue
                ns, name, provider = parts[0], parts[1], parts[2]
                ldir = os.path.join(hdir, leaf)
                if not os.path.isdir(ldir):
                    continue
                for d in sorted(os.listdir(ldir)):
                    if not d.startswith('v'):
                        continue
                    ver = d[1:]
                    vdir = os.path.join(ldir, d)
                    content, ctype, title = pick_content(vdir)
                    if not content:
                        continue
                    url = f"https://{host}/modules/{ns}/{name}/{provider}/{ver}"
                    rec = {
                        'type': 'module',
                        'namespace': ns,
                        'name': name,
                        'version': ver,
                        'url': url,
                        'title': title or f"{name} module",
                        'content': content,
                        'content_type': ctype,
                        'scraped_at': now_iso(),
                    }
                    out.write(json.dumps(rec, ensure_ascii=False) + "\n")
                    count += 1
    return count


def main():
    ap = argparse.ArgumentParser(description='Export ingest corpus to JSONL for normalizer')
    ap.add_argument('--root', default='artifacts/ingest_full2')
    ap.add_argument('--out', default='_to_review/terraform_db_export')
    args = ap.parse_args()
    os.makedirs(args.out, exist_ok=True)
    prov_out = os.path.join(args.out, 'providers.jsonl')
    mod_out = os.path.join(args.out, 'modules.jsonl')
    pc = export_providers(args.root, prov_out)
    mc = export_modules(args.root, mod_out)
    print(f"exported providers={pc} modules={mc} -> {args.out}")


if __name__ == '__main__':
    main()

