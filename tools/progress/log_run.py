#!/usr/bin/env python3
"""
Append a JSONL progress entry to artifacts/runlog.jsonl.

Usage examples:
  python3 tools/progress/log_run.py --action ingest.batch --status completed --ingest-dir artifacts/ingest_run
  python3 tools/progress/log_run.py --action note --status info --details '{"message":"checkpoint"}'
"""
import argparse
import datetime as dt
import glob
import json
import os
from typing import Any, Dict


def _now_iso() -> str:
    return dt.datetime.utcnow().replace(microsecond=0).isoformat() + "Z"


def load_json(path: str) -> Dict[str, Any]:
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return {}


def synthesize_ingest_details(root: str) -> Dict[str, Any]:
    d: Dict[str, Any] = {"ingest_dir": root}
    summary_path = os.path.join(root, "batch_summary.json")
    if os.path.isfile(summary_path):
        d.update(load_json(summary_path))
    # Find most recent snapshot file in the directory
    snaps = sorted(glob.glob(os.path.join(root, "snapshot-*.json")))
    if snaps:
        d["snapshot"] = snaps[-1]
        # Optionally include tiny header to ease later reading
        try:
            head = load_json(snaps[-1])
            if isinstance(head, dict):
                d["snapshot_generated_at"] = head.get("generated_at")
                # quick metric sprinkling if present
                if isinstance(head.get("metrics"), dict):
                    d["metrics"] = {k: head["metrics"].get(k) for k in ("docs_md_count", "docs_html_count") if k in head["metrics"]}
        except Exception:
            pass
    return d


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--log", default="artifacts/runlog.jsonl")
    p.add_argument("--action", required=True)
    p.add_argument("--status", default="info")
    p.add_argument("--details", default="", help="inline JSON for details")
    p.add_argument("--details-file", default="", help="path to JSON file for details")
    p.add_argument("--ingest-dir", default="", help="point to an ingest batch dir to auto-capture summary + snapshot")
    args = p.parse_args()

    details: Dict[str, Any] = {}
    if args.details.strip():
        try:
            details.update(json.loads(args.details))
        except Exception:
            pass
    if args.details_file.strip() and os.path.isfile(args.details_file):
        details.update(load_json(args.details_file))
    if args.ingest_dir.strip() and os.path.isdir(args.ingest_dir):
        details.update(synthesize_ingest_details(args.ingest_dir))

    entry = {
        "ts": _now_iso(),
        "action": args.action,
        "status": args.status,
        "details": details,
    }

    os.makedirs(os.path.dirname(args.log), exist_ok=True)
    with open(args.log, "ab") as f:
        line = (json.dumps(entry) + "\n").encode("utf-8")
        f.write(line)
    print(args.log)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

