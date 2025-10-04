#!/usr/bin/env python3
import argparse
import asyncio
import json
import os
import sys
from datetime import datetime, timezone
from typing import Dict, Any

import aiohttp
from bs4 import BeautifulSoup


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


async def fetch(session: aiohttp.ClientSession, url: str) -> str:
    async with session.get(url) as r:
        if r.status != 200:
            return ""
        return await r.text()


def extract_main_html(html: str) -> str:
    soup = BeautifulSoup(html, 'html.parser')
    # Try registry main content containers in order
    for sel in [
        'main',
        'article',
        'div.docs-content',
        'div.package-body',
        'div#content',
    ]:
        el = soup.select_one(sel)
        if el:
            return str(el)
    return html


async def scrape_providers(session: aiohttp.ClientSession, versions: Dict[str, Any], out_dir: str):
    prov_out = os.path.join(out_dir, 'providers.jsonl')
    os.makedirs(out_dir, exist_ok=True)
    cnt = 0
    with open(prov_out, 'w', encoding='utf-8') as f:
        for key, meta in versions.items():
            # key format: providers:<ns>:<name>
            parts = key.split(':')
            if len(parts) < 3:
                continue
            _, ns, name = parts[:3]
            # prefer latest version if any
            ver_list = meta.get('versions') or []
            ver = ver_list[0] if ver_list else 'latest'
            url = f"https://registry.terraform.io/providers/{ns}/{name}/{ver}/docs"
            html = await fetch(session, url)
            if not html:
                continue
            content = extract_main_html(html)
            rec = {
                'type': 'provider',
                'namespace': ns,
                'name': name,
                'version': ver if ver != 'latest' else '',
                'url': url,
                'title': f"{name} provider",
                'content': content,
                'content_type': 'html',
                'scraped_at': now_iso(),
            }
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")
            cnt += 1
    return cnt


async def scrape_modules(session: aiohttp.ClientSession, versions: Dict[str, Any], out_dir: str):
    mod_out = os.path.join(out_dir, 'modules.jsonl')
    os.makedirs(out_dir, exist_ok=True)
    cnt = 0
    with open(mod_out, 'w', encoding='utf-8') as f:
        for key, meta in versions.items():
            # key format: modules:<ns>:<name>:<target_system>
            parts = key.split(':')
            if len(parts) < 4:
                continue
            _, ns, name, target = parts[:4]
            ver_list = meta.get('versions') or []
            ver = ver_list[0] if ver_list else 'latest'
            url = f"https://registry.terraform.io/modules/{ns}/{name}/{target}/{ver}"
            html = await fetch(session, url)
            if not html:
                continue
            content = extract_main_html(html)
            rec = {
                'type': 'module',
                'namespace': ns,
                'name': name,
                'version': ver if ver != 'latest' else '',
                'url': url,
                'title': f"{name} module",
                'content': content,
                'content_type': 'html',
                'scraped_at': now_iso(),
            }
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")
            cnt += 1
    return cnt


async def main():
    p = argparse.ArgumentParser(description='Scrape Terraform Registry pages to JSONL exports')
    p.add_argument('--providers', help='discovery JSON for providers')
    p.add_argument('--modules', help='discovery JSON for modules')
    p.add_argument('--out', default='../../_to_review/terraform_db_export', help='output directory for JSONL')
    args = p.parse_args()

    prov_map = {}
    mod_map = {}
    if args.providers and os.path.exists(args.providers):
        prov_map = json.load(open(args.providers))
    if args.modules and os.path.exists(args.modules):
        mod_map = json.load(open(args.modules))

    async with aiohttp.ClientSession(headers={'User-Agent': 'TFUG-Registry-Scraper/1.0'}) as session:
        pcnt = await scrape_providers(session, prov_map, args.out)
        mcnt = await scrape_modules(session, mod_map, args.out)
        sys.stderr.write(f"scraped providers={pcnt} modules={mcnt}\n")


if __name__ == '__main__':
    asyncio.run(main())

