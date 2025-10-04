#!/usr/bin/env python3
import argparse
import asyncio
import json
import os
import re
from datetime import datetime, timezone

import aiohttp
from bs4 import BeautifulSoup


def now_iso():
    return datetime.now(timezone.utc).isoformat()


async def fetch(session, url):
    async with session.get(url) as r:
        if r.status != 200:
            return ""
        return await r.text()


def extract(html):
    soup = BeautifulSoup(html, 'html.parser')
    for sel in ['main','article','div.docs-content','div.package-body','div#content']:
        el = soup.select_one(sel)
        if el:
            return str(el)
    return html


def classify(url):
    # returns (type, namespace, name, version, resource(optional), target(optional))
    m = re.search(r"/providers/([^/]+)/([^/]+)/([^/]+)/docs(?:/(resources|data-sources)/([^/]+))?", url)
    if m:
        ns, name, ver = m.group(1), m.group(2), m.group(3)
        kind = 'provider'
        resource = m.group(5) or ''
        return kind, ns, name, ver, resource, ''
    m = re.search(r"/modules/([^/]+)/([^/]+)/([^/]+)/([^/]+)", url)
    if m:
        ns, name, target, ver = m.group(1), m.group(2), m.group(3), m.group(4)
        return 'module', ns, name, ver, '', target
    return '', '', '', '', '', ''


async def main():
    p = argparse.ArgumentParser()
    p.add_argument('--seeds', default='seeds.txt')
    p.add_argument('--out', default='../../_to_review/terraform_db_export')
    args = p.parse_args()
    seeds = []
    if os.path.exists(args.seeds):
        raw = [line.strip() for line in open(args.seeds) if line.strip() and not line.startswith('#')]
        # Accept either full URLs or artifact IDs like provider:ns/name, module:ns/name/provider
        for s in raw:
            if s.startswith('http://') or s.startswith('https://'):
                seeds.append(s)
                continue
            if s.startswith('provider:'):
                body = s.split(':', 1)[1]
                if '/' in body:
                    ns, name = body.split('/')
                    seeds.append(f'https://registry.terraform.io/providers/{ns}/{name}/latest/docs')
                    continue
            if s.startswith('module:'):
                body = s.split(':', 1)[1]
                parts = body.split('/')
                if len(parts) == 3:
                    ns, name, target = parts
                    seeds.append(f'https://registry.terraform.io/modules/{ns}/{name}/{target}/latest')
                    continue
        # Ignore malformed lines silently to keep pipeline resilient
    os.makedirs(args.out, exist_ok=True)
    prov_f = open(os.path.join(args.out,'providers.jsonl'),'w',encoding='utf-8')
    mod_f = open(os.path.join(args.out,'modules.jsonl'),'w',encoding='utf-8')
    try:
        async with aiohttp.ClientSession(headers={'User-Agent':'TFUG-Seed-Scraper/1.0'}) as session:
            for url in seeds:
                html = await fetch(session, url)
                if not html:
                    continue
                content = extract(html)
                kind, ns, name, ver, resource, target = classify(url)
                if not kind:
                    continue
                if kind == 'provider':
                    rec = {
                        'type':'provider','namespace':ns,'name':name,'version':ver if ver!='latest' else '',
                        'url':url,'title':f'{name} provider','content':content,'content_type':'html','scraped_at':now_iso()
                    }
                    prov_f.write(json.dumps(rec, ensure_ascii=False)+"\n")
                elif kind == 'module':
                    rec = {
                        'type':'module','namespace':ns,'name':name,'version':ver if ver!='latest' else '',
                        'url':url,'title':f'{name} module','content':content,'content_type':'html','scraped_at':now_iso()
                    }
                    mod_f.write(json.dumps(rec, ensure_ascii=False)+"\n")
    finally:
        prov_f.close()
        mod_f.close()

if __name__ == '__main__':
    asyncio.run(main())
