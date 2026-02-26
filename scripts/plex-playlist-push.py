#!/usr/bin/env python3
"""
Push M3U playlists from a music library to Plex.

Obtains a Plex token via PIN auth (if not provided), finds all .m3u files
in the given path, and uploads each to the Plex server via the playlists API.

Usage:
  plex-playlist-push.py --server URL --path PATH [--token TOKEN]

Examples:
  # Same host as Plex (path matches):
  plex-playlist-push.py --server http://192.168.50.42:32400 \\
    --path /mnt/peace-house-storage-pool/peace-house-storage/Music

  # Plex in Docker (path as seen by Plex may differ):
  plex-playlist-push.py --server http://192.168.50.42:32400 \\
    --path /mnt/peace-house-storage-pool/peace-house-storage/Music \\
    --plex-path /data

  # With token (skip PIN flow):
  plex-playlist-push.py --server http://192.168.50.42:32400 \\
    --path /mnt/peace-house-storage-pool/peace-house-storage/Music \\
    --token YOUR_PLEX_TOKEN
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import uuid
from pathlib import Path
import urllib.error
import urllib.parse
import urllib.request


def log(msg: str) -> None:
    """Print message to stderr."""
    print(msg, file=sys.stderr, flush=True)


def request(
    url: str,
    method: str = "GET",
    headers: dict[str, str] | None = None,
    data: bytes | None = None,
) -> tuple[int, dict | str]:
    """
    Perform HTTP request and return (status_code, body).
    Body is parsed as JSON if possible, otherwise raw string.
    """
    req_headers = {"Accept": "application/json", **(headers or {})}
    req = urllib.request.Request(url, data=data, headers=req_headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
            try:
                return resp.status, json.loads(body)
            except json.JSONDecodeError:
                return resp.status, body
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8") if e.fp else ""
        try:
            return e.code, json.loads(body)
        except json.JSONDecodeError:
            return e.code, body
    except urllib.error.URLError as e:
        log(f"Request failed: {e}")
        raise SystemExit(1) from e


def get_plex_headers() -> dict[str, str]:
    """Return standard Plex API headers."""
    return {
        "X-Plex-Product": "musicdl-plex-playlist-push",
        "X-Plex-Version": "1.0",
        "X-Plex-Client-Identifier": str(uuid.uuid4()),
        "X-Plex-Platform": "Python",
        "X-Plex-Platform-Version": "3",
        "X-Plex-Device": "Script",
        "X-Plex-Device-Name": "musicdl plex-playlist-push",
    }


def obtain_token_via_pin() -> str:
    """
    Obtain a Plex auth token via the PIN flow.
    User must visit https://app.plex.tv/link and enter the displayed PIN.
    """
    headers = get_plex_headers()
    # Create PIN
    status, body = request(
        "https://plex.tv/pins?strong=true",
        method="POST",
        headers=headers,
    )
    if status not in (200, 201):
        log(f"Failed to create PIN: HTTP {status}")
        if isinstance(body, dict):
            log(json.dumps(body, indent=2))
        else:
            log(str(body))
        raise SystemExit(1)

    pin = body.get("pin", body) if isinstance(body, dict) else {}
    pin_id = pin.get("id")
    code = pin.get("code", "????")
    if not pin_id:
        log("PIN response missing id")
        raise SystemExit(1)

    log("")
    log("To obtain a Plex token, complete the following steps:")
    log("  1. Open https://app.plex.tv/link in your browser")
    log(f"  2. Enter this PIN: {code}")
    log("  3. Wait for this script to detect authorization...")
    log("")

    # Poll for auth
    auth_url = f"https://plex.tv/pins/{pin_id}.json"
    for _ in range(120):  # 2 minutes
        status, resp = request(auth_url, headers=headers)
        if status != 200:
            time.sleep(2)
            continue
        pin_resp = (resp.get("pin") or resp) if isinstance(resp, dict) else {}
        auth_token = pin_resp.get("authToken") or pin_resp.get("auth_token")
        if auth_token:
            log("Authorization successful.")
            return auth_token
        time.sleep(2)

    log("Timeout waiting for PIN authorization.")
    raise SystemExit(1)


def find_m3u_files(path: Path) -> list[Path]:
    """Find all .m3u files under path recursively."""
    if not path.is_dir():
        return []
    return sorted(path.rglob("*.m3u"))


def upload_playlist(server_url: str, token: str, m3u_path: str) -> bool:
    """
    Upload a single M3U file to Plex.
    Returns True on success, False on failure.
    """
    base = server_url.rstrip("/")
    url = f"{base}/playlists/upload?path={urllib.parse.quote(m3u_path, safe='/')}"
    headers = {**get_plex_headers(), "X-Plex-Token": token}
    status, _ = request(url, method="POST", headers=headers)
    return status == 200


def main() -> None:
    """Run the plex-playlist-push script."""
    parser = argparse.ArgumentParser(
        description="Push M3U playlists from a music library to Plex.",
        epilog=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--server",
        required=True,
        help="Plex server URL (e.g. http://192.168.50.42:32400)",
    )
    parser.add_argument(
        "--path",
        required=True,
        help="Path to music library (must exist locally for scanning).",
    )
    parser.add_argument(
        "--plex-path",
        default=None,
        help="Path prefix for Plex (if different from --path, e.g. Docker mount). "
        "Example: --path /mnt/nas/Music --plex-path /data",
    )
    parser.add_argument(
        "--token",
        default=os.environ.get("PLEX_TOKEN"),
        help="Plex auth token (or set PLEX_TOKEN env). If omitted, uses PIN flow.",
    )
    args = parser.parse_args()

    server_url = args.server.rstrip("/")
    lib_path = Path(args.path).resolve()
    plex_path_prefix = args.plex_path.rstrip("/") if args.plex_path else None

    if not lib_path.exists():
        log(f"Path does not exist: {lib_path}")
        raise SystemExit(1)

    # Obtain token
    token = args.token
    if not token:
        log("No token provided. Starting PIN authentication...")
        token = obtain_token_via_pin()
    else:
        log("Using provided token.")

    # Find M3U files
    m3u_files = find_m3u_files(lib_path)
    if not m3u_files:
        log(f"No .m3u files found under {lib_path}")
        raise SystemExit(0)

    log(f"Found {len(m3u_files)} M3U file(s). Uploading...")

    success = 0
    failed = 0
    lib_path_str = str(lib_path)
    for m3u in m3u_files:
        m3u_str = str(m3u)
        if plex_path_prefix:
            # Translate path for Plex (e.g. Docker mount)
            rel = m3u_str[len(lib_path_str) :].lstrip("/")
            m3u_str = f"{plex_path_prefix}/{rel}" if rel else plex_path_prefix
        if upload_playlist(server_url, token, m3u_str):
            log(f"  OK: {m3u.name}")
            success += 1
        else:
            log(f"  FAIL: {m3u.name}")
            failed += 1

    log("")
    log(f"Done. {success} uploaded, {failed} failed.")
    if failed:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
