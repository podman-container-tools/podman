#!/usr/bin/env python3
"""
Script to regenerate Podman release notes from GitHub releases.
"""

import sys
import os
import time
import re
import json
import urllib.request
import urllib.error
from packaging import version as pkg_version

# Configuration
OWNER = "podman-container-tools"
REPO = "podman"
MIN_VERSION = "v0.8.4"
OUTPUT_FILE = "RELEASE_NOTES.md"
API_BASE = "https://api.github.com/repos"

# Historical GitHub locations for this project. It was originally hosted at
# containers/libpod, moved to containers/podman, and now lives at OWNER/REPO.
# Links left over from older releases still point at the old paths, so rewrite
# them to the current location when compiling the notes.
URL_MIGRATIONS = [
    (r"github\.com/containers/libpod(?![\w-])", f"github.com/{OWNER}/{REPO}"),
    (r"github\.com/containers/podman(?![\w-])", f"github.com/{OWNER}/{REPO}"),
]

def compare_versions(v1, v2):
    """Compare two version strings, handling the 'v' prefix."""
    v1_clean = v1.lstrip('v')
    v2_clean = v2.lstrip('v')
    return pkg_version.parse(v1_clean) >= pkg_version.parse(v2_clean)

def has_hyphen(tag):
    """Check if a tag contains a hyphen (excluding the 'v' prefix)."""
    tag_clean = tag.lstrip('v')
    return '-' in tag_clean

def normalize_headers(body):
    """
    Normalize markdown headers in release body.
    Converts any ## headings to ### headings.
    """
    lines = body.split('\n')
    normalized_lines = []

    for line in lines:
        if re.match(r'^##(?!\s*#)', line):
            line = '###' + line[2:]
        normalized_lines.append(line)

    return '\n'.join(normalized_lines)

def rewrite_urls(text):
    """Rewrite historical GitHub URLs to the current repository location."""
    for pattern, replacement in URL_MIGRATIONS:
        text = re.sub(pattern, replacement, text, flags=re.IGNORECASE)
    return text

def check_rate_limit(headers):
    """
    Check GitHub API rate limit headers and return wait time in seconds.
    """
    retry_after = headers.get('Retry-After')
    if retry_after:
        return max(1, int(retry_after))

    remaining = headers.get('X-RateLimit-Remaining')
    if remaining and int(remaining) <= 5:
        reset = headers.get('X-RateLimit-Reset')
        if reset:
            import datetime
            reset_time = datetime.datetime.fromtimestamp(int(reset))
            now = datetime.datetime.now(datetime.timezone.utc)
            wait_seconds = (reset_time - now).total_seconds()
            return max(1, int(wait_seconds) + 1)

    return 0

def fetch_releases_page(page_num, per_page=100):
    """Fetch a page of releases from GitHub API."""
    url = f"{API_BASE}/{OWNER}/{REPO}/releases?per_page={per_page}&page={page_num}"

    req = urllib.request.Request(url)
    req.add_header('Accept', 'application/vnd.github.v3+json')
    req.add_header('User-Agent', 'Podman-Release-Notes-Generator')

    try:
        with urllib.request.urlopen(req) as response:
            headers = response.headers
            data = json.loads(response.read().decode('utf-8'))

            wait_time = check_rate_limit(headers)
            if wait_time > 0:
                print(f"Rate limit approaching, waiting {wait_time} seconds...")
                time.sleep(wait_time)

            return data
    except urllib.error.HTTPError as e:
        if e.code == 429:
            retry_after = e.headers.get('Retry-After', '60')
            wait_time = max(1, int(retry_after))
            print(f"Rate limited! Waiting {wait_time} seconds...")
            time.sleep(wait_time)
            return []
        elif e.code == 403:
            print(f"Error: Forbidden. Check your access to the repository.")
            sys.exit(1)
        else:
            print(f"HTTP Error {e.code}: {e.reason}")
            sys.exit(1)
    except Exception as e:
        print(f"Error fetching releases: {e}")
        sys.exit(1)

def fetch_all_releases():
    """Fetch all releases from GitHub API, handling pagination."""
    all_releases = []
    page = 1

    while True:
        print(f"Fetching page {page}...")
        releases = fetch_releases_page(page)

        if not releases:
            break

        all_releases.extend(releases)

        if len(releases) < 100:
            break

        page += 1
        time.sleep(1)

    return all_releases

def generate_release_notes(releases):
    """Generate release notes markdown from filtered releases."""
    lines = ["# Release Notes", ""]

    for release in releases:
        tag_name = release['tag_name']
        body = release['body']

        if has_hyphen(tag_name):
            continue

        if not compare_versions(tag_name, MIN_VERSION):
            continue

        version_header = tag_name.lstrip('v')
        lines.append(f"## {version_header}")

        normalized_body = rewrite_urls(normalize_headers(body)).replace('\r\n', '\n').replace('\n\n\n', '\n\n').lstrip('\n').rstrip('\n')
        lines.append(normalized_body)
        lines.append("")

    return '\n'.join(lines)

def main():
    if os.path.exists(OUTPUT_FILE):
        print(f"Error: {OUTPUT_FILE} already exists. Exiting without modifying.")
        sys.exit(1)

    print(f"Fetching releases from {OWNER}/{REPO}...")
    releases = fetch_all_releases()
    print(f"Fetched {len(releases)} total releases.")

    # Sort releases by tag name in descending order (newest first)
    def get_version_key(release):
        return pkg_version.parse(release['tag_name'].lstrip('v'))

    releases.sort(key=get_version_key, reverse=True)

    print("Generating release notes...")
    release_notes = generate_release_notes(releases)

    with open(OUTPUT_FILE, 'w', encoding='utf-8') as f:
        f.write(release_notes)

    print(f"Successfully generated {OUTPUT_FILE}")

if __name__ == "__main__":
    main()
