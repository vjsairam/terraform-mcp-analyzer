from html.parser import HTMLParser
from typing import Dict


class _TextExtractor(HTMLParser):
    def __init__(self):
        super().__init__()
        self.out = []

    def handle_starttag(self, tag, attrs):
        if tag in ("p", "div", "section", "br", "h1", "h2", "h3", "h4"):
            self.out.append("\n\n")

    def handle_data(self, data):
        self.out.append(data)

    def get_text(self):
        return "".join(self.out)


def html_to_markdown(html: bytes) -> Dict[str, bytes]:
    # Minimal, safe conversion keeping plaintext structure. Users can plug better converters later.
    p = _TextExtractor()
    try:
        p.feed(html.decode("utf-8", errors="ignore"))
    except Exception:
        return {"doc.md": html}
    text = p.get_text()
    md = text.strip().encode("utf-8")
    return {"doc.md": md}

