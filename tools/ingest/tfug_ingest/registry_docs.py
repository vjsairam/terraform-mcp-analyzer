import time
import urllib.request
import urllib.error
from typing import Optional, Dict


def fetch_html(url: str, etag: Optional[str] = None, last_modified: Optional[str] = None, timeout: int = 30) -> Dict:
    headers = {"User-Agent": "TFUG-Ingest/1.0"}
    if etag:
        headers["If-None-Match"] = etag
    if last_modified:
        headers["If-Modified-Since"] = last_modified
    req = urllib.request.Request(url, headers=headers)
    # Retry with exponential backoff for transient errors
    backoff = 1.0
    for attempt in range(4):
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                status = resp.status
                html = resp.read()
                etag_new = resp.headers.get("ETag")
                lm_new = resp.headers.get("Last-Modified")
                return {"status": status, "html": html, "etag": etag_new, "last_modified": lm_new}
        except urllib.error.HTTPError as e:
            if e.code == 304:
                return {"status": 304}
            # Backoff on 429/5xx
            if e.code in (429, 500, 502, 503, 504) and attempt < 3:
                time.sleep(backoff)
                backoff *= 2
                continue
            raise
        except urllib.error.URLError:
            if attempt < 3:
                time.sleep(backoff)
                backoff *= 2
                continue
            raise
