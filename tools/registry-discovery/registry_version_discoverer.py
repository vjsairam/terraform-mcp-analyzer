"""
Registry Version Discoverer for comprehensive versioned content scraping.
Handles Terraform Registry modules, providers, and policies with all versions.
"""
import asyncio
import aiohttp
import re
from typing import Dict, List, Optional
from urllib.parse import urljoin
from bs4 import BeautifulSoup
from dataclasses import dataclass
import json
import logging

logger = logging.getLogger(__name__)

@dataclass 
class VersionedResource:
    """Represents a versioned resource in the registry"""
    base_url: str
    resource_type: str  # module, provider, policy
    namespace: str
    name: str
    target_system: Optional[str] = None  # aws, azure, etc
    versions: List[str] = None
    
    def __post_init__(self):
        if self.versions is None:
            self.versions = []

class RegistryVersionDiscoverer:
    """Discovers all versions of resources in Terraform Registry"""
    
    def __init__(self, max_concurrent: int = 5, rate_limit_delay: float = 0.5):
        self.max_concurrent = max_concurrent
        self.rate_limit_delay = rate_limit_delay
        self.session: Optional[aiohttp.ClientSession] = None
        self.discovered_resources: Dict[str, VersionedResource] = {}
        
    async def __aenter__(self):
        self.session = aiohttp.ClientSession(
            timeout=aiohttp.ClientTimeout(total=30),
            headers={'User-Agent': 'TFUG-Registry-Discoverer/1.0'}
        )
        return self
        
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()

    async def discover_registry_resources(self, 
                                        resource_types: List[str] = None,
                                        max_pages: int = 5) -> Dict[str, VersionedResource]:
        if resource_types is None:
            resource_types = ['modules', 'providers']
        for resource_type in resource_types:
            await self._discover_from_browse_pages(resource_type, max_pages)
        return self.discovered_resources

    async def _discover_from_browse_pages(self, resource_type: str, max_pages: int):
        base_browse_url = f"https://registry.terraform.io/browse/{resource_type}"
        for page in range(1, max_pages + 1):
            try:
                page_url = f"{base_browse_url}?page={page}"
                if not self.session:
                    continue
                async with self.session.get(page_url) as response:
                    if response.status != 200:
                        logger.warning(f"Browse page {page} returned {response.status}")
                        break
                    content = await response.text()
                    soup = BeautifulSoup(content, 'html.parser')
                    for resource_url in self._extract_resource_links(soup, resource_type):
                        await self._discover_resource_versions(resource_url, resource_type)
                        await asyncio.sleep(self.rate_limit_delay)
                await asyncio.sleep(self.rate_limit_delay)
            except Exception as e:
                logger.error(f"Error processing browse page {page} for {resource_type}: {e}")

    def _extract_resource_links(self, soup: BeautifulSoup, resource_type: str) -> List[str]:
        links = []
        selectors = {
            'modules': ['a[href*="/modules/"]', '.module-card a', '.package-card a'],
            'providers': ['a[href*="/providers/"]', '.provider-card a', '.package-card a'],
        }
        for selector in selectors.get(resource_type, []):
            for link in soup.select(selector):
                href = link.get('href')
                if href and self._is_valid_resource_link(href, resource_type):
                    links.append(urljoin('https://registry.terraform.io', href))
        return list(dict.fromkeys(links))

    def _is_valid_resource_link(self, href: str, resource_type: str) -> bool:
        patterns = {
            'modules': r'/modules/[^/]+/[^/]+/[^/]+',
            'providers': r'/providers/[^/]+/[^/]+'
        }
        pattern = patterns.get(resource_type)
        return bool(re.search(pattern, href)) if pattern else False

    async def _discover_resource_versions(self, resource_url: str, resource_type: str):
        try:
            if not self.session:
                return
            async with self.session.get(resource_url) as response:
                if response.status != 200:
                    return
                content = await response.text()
                soup = BeautifulSoup(content, 'html.parser')
                info = self._parse_resource_url(resource_url, resource_type)
                if not info:
                    return
                versions = self._extract_versions_from_page(soup)
                key = f"{resource_type}:{info['namespace']}:{info['name']}"
                if info.get('target_system'):
                    key += f":{info['target_system']}"
                self.discovered_resources[key] = VersionedResource(
                    base_url=resource_url,
                    resource_type=resource_type,
                    namespace=info['namespace'],
                    name=info['name'],
                    target_system=info.get('target_system'),
                    versions=versions,
                )
        except Exception as e:
            logger.error(f"Error discovering versions for {resource_url}: {e}")

    def _parse_resource_url(self, url: str, resource_type: str) -> Optional[Dict[str, str]]:
        if resource_type == 'modules':
            m = re.search(r'/modules/([^/]+)/([^/]+)/([^/]+)', url)
            if m:
                return {'namespace': m.group(1), 'name': m.group(2), 'target_system': m.group(3)}
        elif resource_type == 'providers':
            m = re.search(r'/providers/([^/]+)/([^/]+)', url)
            if m:
                return {'namespace': m.group(1), 'name': m.group(2)}
        return None

    def _extract_versions_from_page(self, soup: BeautifulSoup) -> List[str]:
        versions: List[str] = []
        selectors = [
            'select[data-testid="version-selector"] option',
            '.version-selector option',
            'select.version-dropdown option',
            'a[href*="/versions/"]',
        ]
        for selector in selectors:
            for el in soup.select(selector):
                version = None
                for attr in ['value', 'data-version', 'href']:
                    if el.has_attr(attr):
                        m = re.search(r'(\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?)', el[attr])
                        if m:
                            version = m.group(1)
                            break
                if not version:
                    text = el.get_text().strip()
                    m = re.search(r'(\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?)', text)
                    if m:
                        version = m.group(1)
                if version:
                    versions.append(version)
        # Deduplicate
        return list(dict.fromkeys([v.strip() for v in versions if v.strip()]))

    def export(self, filename: str):
        export_data = {k: vars(v) for k, v in self.discovered_resources.items()}
        with open(filename, 'w') as f:
            json.dump(export_data, f, indent=2)

