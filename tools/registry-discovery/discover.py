#!/usr/bin/env python3
import argparse
import asyncio
from registry_version_discoverer import RegistryVersionDiscoverer


async def main():
    parser = argparse.ArgumentParser(description="Discover Terraform Registry resources and versions")
    parser.add_argument('--type', choices=['modules', 'providers'], default='modules')
    parser.add_argument('--pages', type=int, default=2)
    parser.add_argument('--out', type=str, default='versions.json')
    args = parser.parse_args()

    async with RegistryVersionDiscoverer() as d:
        await d.discover_registry_resources([args.type], max_pages=args.pages)
        d.export(args.out)

if __name__ == '__main__':
    asyncio.run(main())

