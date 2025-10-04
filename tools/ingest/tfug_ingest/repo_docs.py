from typing import List, Tuple
import os
import json
import time
import urllib.request
import urllib.parse
import urllib.error


def fetch_tree(repo_url: str, ref: str) -> List[Tuple[str, bytes]]:
    """
    Fetch repository docs for a given tag/commit.
    Minimal stub: real implementations should use provider-specific hosting APIs.
    Returns a list of (relative_path, content_bytes) under docs/.
    """
    # Supports GitHub repos: https://github.com/{owner}/{repo}
    # Lists files via Git Trees API and fetches Markdown under docs/ and website/docs/ plus root README.md
    try:
        owner, repo = _parse_github(repo_url)
        if not owner:
            return []
        token = os.environ.get("GITHUB_TOKEN", "")
        headers = {"User-Agent": "TFUG-Ingest/1.0"}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        api = f"https://api.github.com/repos/{owner}/{repo}/git/trees/{urllib.parse.quote(ref, safe='') }?recursive=1"
        req = urllib.request.Request(api, headers=headers)
        data = None
        backoff = 1.0
        for attempt in range(4):
            try:
                with urllib.request.urlopen(req, timeout=30) as resp:
                    # Rate limit awareness
                    rlim = resp.headers.get("X-RateLimit-Remaining")
                    if rlim is not None:
                        try:
                            rem = int(rlim)
                            if rem <= 1:
                                time.sleep(2.0)
                        except ValueError:
                            pass
                    data = json.loads(resp.read().decode("utf-8"))
                    break
            except urllib.error.HTTPError as e:
                if e.code in (403, 429, 500, 502, 503, 504) and attempt < 3:
                    time.sleep(backoff)
                    backoff *= 2
                    continue
                raise
        tree = data.get("tree", [])
        paths = []
        for node in tree:
            p = node.get("path", "")
            if not p.lower().endswith(".md"):
                continue
            # Accept README.md (root) and docs trees
            if p.lower() == "readme.md" or p.startswith("docs/") or p.startswith("website/docs/"):
                paths.append(p)
        files: List[Tuple[str, bytes]] = []
        for p in paths:
            raw = f"https://raw.githubusercontent.com/{owner}/{repo}/{ref}/{p}"
            rreq = urllib.request.Request(raw, headers=headers)
            content = None
            backoff = 1.0
            for attempt in range(4):
                try:
                    with urllib.request.urlopen(rreq, timeout=30) as rresp:
                        rh = rresp.headers
                        rlim = rh.get("X-RateLimit-Remaining")
                        if rlim is not None:
                            try:
                                rem = int(rlim)
                                if rem <= 1:
                                    time.sleep(2.0)
                            except ValueError:
                                pass
                        content = rresp.read()
                        break
                except urllib.error.HTTPError as e:
                    if e.code in (403, 429, 500, 502, 503, 504) and attempt < 3:
                        time.sleep(backoff)
                        backoff *= 2
                        continue
                    raise
            pl = p.lower()
            if pl.startswith("docs/"):
                rel = pl[len("docs/"):]
            elif pl.startswith("website/docs/"):
                rel = pl[len("website/docs/"):]
            else:
                # e.g., README.md at root
                rel = pl
            files.append((rel, content))
        return files
    except Exception:
        return []


def _parse_github(url: str):
    url = url.strip().rstrip("/")
    for prefix in ("https://github.com/", "http://github.com/", "git@github.com:"):
        if url.startswith(prefix):
            rem = url[len(prefix):]
            parts = rem.split("/")
            if len(parts) >= 2:
                repo = parts[1]
                if repo.endswith(".git"):
                    repo = repo[:-4]
                return parts[0], repo
    return "", ""
