#!/usr/bin/env bash
# Extract YouTube-capable cookies from Vivaldi into a Netscape-format file for musicdl / yt-dlp.
# Requires: yt-dlp on PATH (pip install -U yt-dlp), Vivaldi with a logged-in Google session.
#
# Usage:
#   ./export-vivaldi-cookies.sh [output-file]
#
# Env:
#   MUSICDL_COOKIES_OUT  default output path if no argument (default: ./youtube-cookies.txt)
#   MUSICDL_BROWSER      browser key for yt-dlp (default: vivaldi)
#   YOUTUBE_PROBE_URL    URL used to trigger cookie dump (default: a short public YouTube video)
#
# On macOS you may see "Extracting cookies from vivaldi" while Keychain Access allows decryption.

set -euo pipefail

readonly DEFAULT_URL="${YOUTUBE_PROBE_URL:-https://www.youtube.com/watch?v=jNQXAC9IVRw}"
readonly BROWSER="${MUSICDL_BROWSER:-vivaldi}"

out_path="${1:-${MUSICDL_COOKIES_OUT:-./youtube-cookies.txt}}"

if ! command -v yt-dlp >/dev/null 2>&1; then
	echo "error: yt-dlp not found. Install with: pip install -U yt-dlp" >&2
	exit 1
fi

target_dir="$(cd "$(dirname "${out_path}")" && pwd)"
target_file="$(basename "${out_path}")"
target="${target_dir}/${target_file}"

echo "Writing Netscape cookie jar to: ${target}"
echo "Browser: ${BROWSER} (set MUSICDL_BROWSER to override)"
echo ""

yt-dlp \
	--cookies-from-browser "${BROWSER}" \
	--cookies "${target}" \
	--skip-download \
	--no-warnings \
	"${DEFAULT_URL}"

chmod 600 "${target}"
echo ""
echo "Done. Point musicdl at this path in config, for example:"
echo "  download:"
echo "    cookies: \"/path/in/container/cookies.txt\""
echo ""
echo "Mount the file read-only in Docker, e.g. -v ${target}:/download/cookies.txt:ro"
