from dataclasses import dataclass
from typing import Optional, List, Literal, Dict

ArtifactType = Literal["provider", "module"]


@dataclass
class ProviderRef:
    host: str
    namespace: str
    name: str
    source_id: str  # "provider:<namespace>/<name>"


@dataclass
class ModuleRef:
    host: str
    namespace: str
    name: str
    provider: str
    source_id: str  # "module:<namespace>/<name>/<provider>"


@dataclass
class VersionMeta:
    artifact: str
    version: str
    host: str
    download_url: Optional[str] = None
    docs_url: Optional[str] = None
    repo_url: Optional[str] = None
    release_notes_url: Optional[str] = None
    fetched: Dict = None
    digests: Dict = None

