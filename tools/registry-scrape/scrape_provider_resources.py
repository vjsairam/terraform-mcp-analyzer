#!/usr/bin/env python3
"""
Scrape provider resource and data source documentation pages from
registry.terraform.io into JSONL files compatible with TFUG normalizers.

Inputs:
  --provider hashicorp/aws            Namespace/name of provider
  --version 5.0.0                     Specific version (default: latest)
  --out _to_review/terraform_db_export

Outputs:
  - providers_resources.jsonl
  - providers_data_sources.jsonl

Notes:
  - Uses HTML scraping of the provider docs index to discover links.
  - Keeps requests polite and handles transient errors.
  - Designed for local artifact generation; not used by tfug scan path.
"""
import argparse
import asyncio
import json
import os
import re
from datetime import datetime, timezone
from typing import List, Tuple, Dict

import aiohttp
from bs4 import BeautifulSoup


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def extract_main_html(html: str) -> str:
    soup = BeautifulSoup(html, 'html.parser')
    for sel in ['main', 'article', 'div.docs-content', 'div.package-body', 'div#content']:
        el = soup.select_one(sel)
        if el:
            return str(el)
    return html


async def fetch_text(session: aiohttp.ClientSession, url: str) -> str:
    for _ in range(4):
        try:
            async with session.get(url) as r:
                if r.status == 200:
                    return await r.text()
                if r.status in (429, 500, 502, 503, 504):
                    await asyncio.sleep(0.5)
                    continue
                return ""
        except Exception:
            await asyncio.sleep(0.5)
    return ""


def discover_links(index_html: str, base: str) -> Tuple[List[str], List[str]]:
    """
    Returns two lists of absolute URLs: (resources, data_sources)
    """
    soup = BeautifulSoup(index_html, 'html.parser')
    res: List[str] = []
    dsrc: List[str] = []
    for a in soup.find_all('a', href=True):
        href = a['href']
        if not href.startswith('/'):
            continue
        if '/docs/resources/' in href:
            if href.startswith(base):
                res.append('https://registry.terraform.io' + href)
        if '/docs/data-sources/' in href:
            if href.startswith(base):
                dsrc.append('https://registry.terraform.io' + href)
    # Deduplicate while preserving order
    def dedup(xs: List[str]) -> List[str]:
        seen = set()
        out = []
        for x in xs:
            if x not in seen:
                seen.add(x)
                out.append(x)
        return out
    return dedup(res), dedup(dsrc)


def classify_slug(url: str) -> str:
    # Extract the resource/data source slug from the URL tail
    m = re.search(r'/docs/(?:resources|data-sources)/([^/?#]+)', url)
    return m.group(1) if m else ""


async def scrape_provider(session: aiohttp.ClientSession, ns: str, name: str, ver: str, out_dir: str) -> Dict[str, int]:
    index_url = f"https://registry.terraform.io/providers/{ns}/{name}/{ver}/docs"
    index_html = await fetch_text(session, index_url)
    if not index_html:
        return {"resources": 0, "data_sources": 0}
    base = f"/providers/{ns}/{name}/{ver}/docs"
    res_links, dsrc_links = discover_links(index_html, base)

    os.makedirs(out_dir, exist_ok=True)
    res_path = os.path.join(out_dir, 'providers_resources.jsonl')
    dsrc_path = os.path.join(out_dir, 'providers_data_sources.jsonl')

    res_count = 0
    dsrc_count = 0

    async def scrape_one(url: str, kind: str) -> Dict:
        html = await fetch_text(session, url)
        if not html:
            return {}
        content = extract_main_html(html)
        slug = classify_slug(url)
        entry = {
            'type': 'provider_resource' if kind == 'resource' else 'provider_data_source',
            'namespace': ns,
            'name': name,
            'version': ver if ver != 'latest' else '',
            'url': url,
            'title': slug,
            'content': content,
            'content_type': 'html',
            'scraped_at': now_iso(),
            'resource': slug,
        }
        return entry

    # Scrape with mild concurrency
    sem = asyncio.Semaphore(6)

    async def limited(entry_coro):
        async with sem:
            return await entry_coro

    tasks = [limited(scrape_one(u, 'resource')) for u in res_links]
    tasks += [limited(scrape_one(u, 'data')) for u in dsrc_links]
    results = await asyncio.gather(*tasks)

    with open(res_path, 'a', encoding='utf-8') as rf, open(dsrc_path, 'a', encoding='utf-8') as df:
        for e in results:
            if not e:
                continue
            line = json.dumps(e, ensure_ascii=False) + "\n"
            if e['type'] == 'provider_resource':
                rf.write(line)
                res_count += 1
            else:
                df.write(line)
                dsrc_count += 1
    return {"resources": res_count, "data_sources": dsrc_count}


async def main():
    ap = argparse.ArgumentParser(description='Scrape provider resource/data-source pages to JSONL')
    ap.add_argument('--provider', required=True, help='namespace/name, e.g. hashicorp/aws')
    ap.add_argument('--version', default='latest', help='version (e.g., 5.0.0) or latest')
    ap.add_argument('--out', default='../../_to_review/terraform_db_export', help='output directory')
    args = ap.parse_args()

    ns, name = args.provider.split('/') if '/' in args.provider else (args.provider, '')
    ver = args.version

    headers = {'User-Agent': 'TFUG-Registry-Resources/1.0'}
    async with aiohttp.ClientSession(headers=headers) as session:
        counts = await scrape_provider(session, ns, name, ver, args.out)
        print(json.dumps({"provider": args.provider, "version": ver, **counts}))


if __name__ == '__main__':
    asyncio.run(main())

