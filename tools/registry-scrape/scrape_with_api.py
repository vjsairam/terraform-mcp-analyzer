#!/usr/bin/env python3
"""
Enhanced scraper that uses the Terraform Registry API to discover resources
and attempts to fetch documentation content.
"""
import argparse
import asyncio
import json
import os
from datetime import datetime, timezone
import aiohttp
from typing import List, Dict, Optional

def now_iso():
    return datetime.now(timezone.utc).isoformat()

async def fetch_providers_list(session: aiohttp.ClientSession, page_size: int = 10) -> List[Dict]:
    """Fetch list of providers from the API"""
    providers = []
    url = f"https://registry.terraform.io/v2/providers?page[size]={page_size}"
    
    async with session.get(url) as response:
        if response.status == 200:
            data = await response.json()
            for provider in data.get('data', []):
                providers.append({
                    'namespace': provider['attributes']['namespace'],
                    'name': provider['attributes']['name'],
                    'full_name': provider['attributes']['full-name'],
                    'description': provider['attributes'].get('description', ''),
                    'source': provider['attributes'].get('source', ''),
                })
    return providers

async def fetch_modules_list(session: aiohttp.ClientSession, page_size: int = 10) -> List[Dict]:
    """Fetch list of modules from the API"""
    modules = []
    url = f"https://registry.terraform.io/v2/modules?page[size]={page_size}"
    
    async with session.get(url) as response:
        if response.status == 200:
            data = await response.json()
            for module in data.get('data', []):
                modules.append({
                    'namespace': module['attributes']['namespace'],
                    'name': module['attributes']['name'],
                    'provider': module['attributes'].get('provider-name', 'aws'),
                    'description': module['attributes'].get('description', ''),
                    'source': module['attributes'].get('source', ''),
                })
    return modules

def create_export_entry(resource_type: str, namespace: str, name: str, 
                        version: str = "latest", description: str = "",
                        url: str = "", content: str = "", provider: str = "") -> Dict:
    """Create a standardized export entry"""
    if resource_type == 'provider':
        url = url or f"https://registry.terraform.io/providers/{namespace}/{name}/{version}/docs"
    elif resource_type == 'module':
        provider = provider or 'aws'
        url = url or f"https://registry.terraform.io/modules/{namespace}/{name}/{provider}/{version}"
    
    return {
        'type': resource_type,
        'namespace': namespace,
        'name': name,
        'version': version,
        'url': url,
        'title': f"{name} {resource_type}",
        'content': content or f"<div>{description}</div>" if description else "<div>Documentation not available</div>",
        'content_type': 'html',
        'scraped_at': now_iso()
    }

async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', default='_to_review/terraform_db_export')
    parser.add_argument('--limit', type=int, default=5, help='Number of resources to fetch')
    args = parser.parse_args()

    os.makedirs(args.out, exist_ok=True)
    
    async with aiohttp.ClientSession() as session:
        # Fetch providers
        print(f"Fetching up to {args.limit} providers...")
        providers = await fetch_providers_list(session, args.limit)
        
        # Fetch modules  
        print(f"Fetching up to {args.limit} modules...")
        modules = await fetch_modules_list(session, args.limit)
        
        # Export providers
        providers_file = os.path.join(args.out, 'providers.jsonl')
        with open(providers_file, 'w') as f:
            for provider in providers:
                entry = create_export_entry(
                    'provider',
                    provider['namespace'],
                    provider['name'],
                    description=provider['description']
                )
                f.write(json.dumps(entry) + '\n')
        
        print(f"Exported {len(providers)} providers to {providers_file}")
        
        # Export modules
        modules_file = os.path.join(args.out, 'modules.jsonl')
        with open(modules_file, 'w') as f:
            for module in modules:
                entry = create_export_entry(
                    'module',
                    module['namespace'],
                    module['name'],
                    description=module['description'],
                    provider=module['provider']
                )
                f.write(json.dumps(entry) + '\n')
        
        print(f"Exported {len(modules)} modules to {modules_file}")

if __name__ == '__main__':
    asyncio.run(main())