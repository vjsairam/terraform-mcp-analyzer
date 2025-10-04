#!/usr/bin/env python3
"""
Scraper that fetches documentation from GitHub repositories
linked in the Terraform Registry.
"""
import argparse
import asyncio
import json
import os
import re
from datetime import datetime, timezone
from typing import List, Dict, Optional, Tuple
import aiohttp
from bs4 import BeautifulSoup

def now_iso():
    return datetime.now(timezone.utc).isoformat()

def extract_github_info(source_url: str) -> Tuple[Optional[str], Optional[str]]:
    """Extract owner and repo from GitHub URL"""
    match = re.match(r'https://github\.com/([^/]+)/([^/]+)', source_url)
    if match:
        return match.group(1), match.group(2)
    return None, None

async def fetch_github_readme(session: aiohttp.ClientSession, owner: str, repo: str, branch: str = "main") -> Optional[str]:
    """Fetch README content from GitHub via raw content URL"""
    branches_to_try = [branch, "master", "main"]
    
    for branch in branches_to_try:
        for readme_name in ["README.md", "readme.md", "Readme.md"]:
            url = f"https://raw.githubusercontent.com/{owner}/{repo}/{branch}/{readme_name}"
            try:
                async with session.get(url) as response:
                    if response.status == 200:
                        content = await response.text()
                        return content
            except:
                continue
    
    return None

async def fetch_provider_docs(session: aiohttp.ClientSession, owner: str, repo: str) -> Dict[str, str]:
    """Fetch provider documentation from standard locations"""
    docs = {}
    
    # Try to get main provider docs
    main_doc_paths = [
        "docs/index.md",
        "website/docs/index.html.markdown",
        "website/docs/index.html.md",
        "docs/README.md"
    ]
    
    for path in main_doc_paths:
        url = f"https://raw.githubusercontent.com/{owner}/{repo}/main/{path}"
        try:
            async with session.get(url) as response:
                if response.status == 200:
                    docs['main'] = await response.text()
                    break
        except:
            continue
    
    # If no main docs, try README
    if 'main' not in docs:
        readme = await fetch_github_readme(session, owner, repo)
        if readme:
            docs['main'] = readme
    
    return docs

async def fetch_module_docs(session: aiohttp.ClientSession, owner: str, repo: str) -> Dict[str, str]:
    """Fetch module documentation from standard locations"""
    docs = {}
    
    # Get README (primary documentation for modules)
    readme = await fetch_github_readme(session, owner, repo)
    if readme:
        docs['main'] = readme
    
    # Try to get examples
    example_paths = [
        "examples/README.md",
        "examples/complete/README.md",
        "examples/simple/README.md"
    ]
    
    for path in example_paths:
        url = f"https://raw.githubusercontent.com/{owner}/{repo}/main/{path}"
        try:
            async with session.get(url) as response:
                if response.status == 200:
                    docs['examples'] = await response.text()
                    break
        except:
            continue
    
    return docs

async def fetch_providers_from_api(session: aiohttp.ClientSession, limit: int = 10) -> List[Dict]:
    """Fetch provider list from Terraform Registry API"""
    providers = []
    # Focus on popular/official providers
    url = f"https://registry.terraform.io/v2/providers?filter[tier]=official&page[size]={limit}"
    
    try:
        async with session.get(url) as response:
            if response.status == 200:
                data = await response.json()
                for provider in data.get('data', []):
                    providers.append({
                        'namespace': provider['attributes']['namespace'],
                        'name': provider['attributes']['name'],
                        'source': provider['attributes'].get('source', ''),
                        'description': provider['attributes'].get('description', ''),
                    })
    except Exception as e:
        print(f"Error fetching providers: {e}")
    
    # If no official providers, get popular ones
    if not providers:
        url = f"https://registry.terraform.io/v2/providers?page[size]={limit}"
        try:
            async with session.get(url) as response:
                if response.status == 200:
                    data = await response.json()
                    for provider in data.get('data', []):
                        if provider['attributes'].get('source'):  # Only if source is available
                            providers.append({
                                'namespace': provider['attributes']['namespace'],
                                'name': provider['attributes']['name'],
                                'source': provider['attributes'].get('source', ''),
                                'description': provider['attributes'].get('description', ''),
                            })
        except Exception as e:
            print(f"Error fetching providers: {e}")
    
    return providers

async def fetch_modules_from_api(session: aiohttp.ClientSession, limit: int = 10) -> List[Dict]:
    """Fetch module list from Terraform Registry API"""
    modules = []
    url = f"https://registry.terraform.io/v2/modules?page[size]={limit}"
    
    try:
        async with session.get(url) as response:
            if response.status == 200:
                data = await response.json()
                for module in data.get('data', []):
                    modules.append({
                        'namespace': module['attributes']['namespace'],
                        'name': module['attributes']['name'],
                        'provider': module['attributes'].get('provider-name', 'aws'),
                        'source': module['attributes'].get('source', ''),
                        'description': module['attributes'].get('description', ''),
                    })
    except Exception as e:
        print(f"Error fetching modules: {e}")
    
    return modules

def markdown_to_html(markdown_content: str) -> str:
    """Simple conversion of markdown to HTML"""
    # This is a basic conversion - in production you'd use a proper markdown parser
    html = f"<div class='markdown-content'>{markdown_content}</div>"
    return html

async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', default='_to_review/terraform_db_export')
    parser.add_argument('--limit', type=int, default=5, help='Number of resources to fetch')
    args = parser.parse_args()

    os.makedirs(args.out, exist_ok=True)
    
    async with aiohttp.ClientSession() as session:
        # Fetch providers
        print(f"Fetching up to {args.limit} providers from Registry API...")
        providers = await fetch_providers_from_api(session, args.limit)
        print(f"Found {len(providers)} providers with GitHub sources")
        
        # Process providers
        providers_file = os.path.join(args.out, 'providers.jsonl')
        with open(providers_file, 'w') as f:
            for provider in providers:
                if provider['source']:
                    owner, repo = extract_github_info(provider['source'])
                    if owner and repo:
                        print(f"Fetching docs for {owner}/{repo}...")
                        docs = await fetch_provider_docs(session, owner, repo)
                        
                        content = docs.get('main', f"<div>{provider['description']}</div>")
                        if content and content.endswith('.md'):
                            content = markdown_to_html(content)
                        
                        entry = {
                            'type': 'provider',
                            'namespace': provider['namespace'],
                            'name': provider['name'],
                            'version': 'latest',
                            'url': f"https://registry.terraform.io/providers/{provider['namespace']}/{provider['name']}/latest/docs",
                            'title': f"{provider['name']} provider",
                            'content': content if content else f"<div>{provider['description']}</div>",
                            'content_type': 'html' if not content.endswith('.md') else 'md',
                            'scraped_at': now_iso(),
                            'github_source': provider['source']
                        }
                        f.write(json.dumps(entry) + '\n')
                        await asyncio.sleep(0.5)  # Rate limiting
        
        print(f"Exported providers to {providers_file}")
        
        # Fetch modules
        print(f"\nFetching up to {args.limit} modules from Registry API...")
        modules = await fetch_modules_from_api(session, args.limit)
        print(f"Found {len(modules)} modules with GitHub sources")
        
        # Process modules
        modules_file = os.path.join(args.out, 'modules.jsonl')
        with open(modules_file, 'w') as f:
            for module in modules:
                if module['source']:
                    owner, repo = extract_github_info(module['source'])
                    if owner and repo:
                        print(f"Fetching docs for {owner}/{repo}...")
                        docs = await fetch_module_docs(session, owner, repo)
                        
                        content = docs.get('main', f"<div>{module['description']}</div>")
                        if docs.get('examples'):
                            content += f"\n\n## Examples\n{docs['examples']}"
                        
                        if content and (content.endswith('.md') or '\n#' in content):
                            content = markdown_to_html(content)
                        
                        entry = {
                            'type': 'module',
                            'namespace': module['namespace'],
                            'name': module['name'],
                            'version': 'latest',
                            'url': f"https://registry.terraform.io/modules/{module['namespace']}/{module['name']}/{module['provider']}/latest",
                            'title': f"{module['name']} module",
                            'content': content if content else f"<div>{module['description']}</div>",
                            'content_type': 'html' if not content.endswith('.md') else 'md',
                            'scraped_at': now_iso(),
                            'github_source': module['source']
                        }
                        f.write(json.dumps(entry) + '\n')
                        await asyncio.sleep(0.5)  # Rate limiting
        
        print(f"Exported modules to {modules_file}")

if __name__ == '__main__':
    asyncio.run(main())