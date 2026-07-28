#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hmac
import json
import os
import subprocess
import sys
import time
from collections.abc import Callable, Iterable
from pathlib import PurePosixPath
from urllib.error import HTTPError, URLError
from urllib.parse import urljoin, urlsplit
from urllib.request import Request
from urllib.request import urlopen as stdlib_urlopen
from xml.etree import ElementTree

from rowset.indexnow import INDEXNOW_KEY_PATH, is_valid_indexnow_key

INDEXNOW_ENDPOINT = "https://api.indexnow.org/indexnow"
MAX_URLS_PER_REQUEST = 10_000
REQUEST_TIMEOUT_SECONDS = 20
REQUEST_RETRY_ATTEMPTS = 3
REQUEST_RETRY_DELAY_SECONDS = 5
RETRYABLE_HTTP_STATUSES = {429, 500, 502, 503, 504}
MAX_SITEMAP_DOCUMENTS = 100
ZERO_SHA = "0" * 40

PUBLIC_CONTENT_PREFIX = ("apps", "pages", "content")
PUBLIC_CONTENT_SECTIONS = {"blog", "docs", "public", "use-cases", "vs"}
PUBLIC_PAGE_PATHS = {
    "blog": "/blog",
    "index": "/",
    "pricing": "/pricing",
    "privacy-policy": "/privacy-policy",
    "terms-of-service": "/terms-of-service",
    "uses": "/uses",
}
GLOBAL_PUBLIC_FILES = {
    "apps/pages/content/navigation.yaml",
    "frontend/templates/base_landing.html",
    "rowset/sitemaps.py",
    "rowset/urls.py",
}
GLOBAL_PUBLIC_PREFIXES = (
    "frontend/templates/blog/",
    "frontend/templates/components/",
    "frontend/templates/pages/",
)

GitChange = tuple[str, tuple[str, ...]]
UrlOpen = Callable[..., object]


class IndexNowError(RuntimeError):
    pass


def parse_name_status(raw_output: bytes) -> list[GitChange]:
    fields = raw_output.decode("utf-8").split("\0")
    if fields and not fields[-1]:
        fields.pop()

    changes = []
    index = 0
    while index < len(fields):
        status = fields[index]
        path_count = 2 if status.startswith(("R", "C")) else 1
        paths = tuple(fields[index + 1 : index + 1 + path_count])
        if len(paths) != path_count:
            raise IndexNowError("Could not parse the git name-status output.")
        changes.append((status, paths))
        index += 1 + path_count
    return changes


def read_git_changes(before: str, after: str) -> list[GitChange]:
    try:
        result = subprocess.run(
            ["git", "diff", "--name-status", "-z", before, after],
            check=True,
            capture_output=True,
        )
    except subprocess.CalledProcessError as exc:
        message = exc.stderr.decode("utf-8", errors="replace").strip()
        raise IndexNowError(f"Could not read deployed git changes: {message}") from exc
    return parse_name_status(result.stdout)


def _content_public_path(changed_path: str) -> str | None:
    path = PurePosixPath(changed_path)
    if path.suffix != ".md" or path.parts[:3] != PUBLIC_CONTENT_PREFIX or len(path.parts) != 5:
        return None

    section = path.parts[3]
    if section not in PUBLIC_CONTENT_SECTIONS:
        return None

    slug = path.stem
    if section == "public":
        return PUBLIC_PAGE_PATHS.get(slug)
    if section == "use-cases" and slug == "index":
        return "/use-cases"
    return f"/{section}/{slug}"


def _is_global_public_change(changed_path: str) -> bool:
    if changed_path in GLOBAL_PUBLIC_FILES:
        return True
    if changed_path.startswith(GLOBAL_PUBLIC_PREFIXES):
        return True

    path = PurePosixPath(changed_path)
    if path.parts[:2] != ("apps", "pages") or path.suffix != ".py":
        return False
    return "tests" not in path.parts and not path.name.startswith("test")


def public_paths_for_changes(changes: Iterable[GitChange]) -> tuple[set[str], bool]:
    public_paths = set()
    include_sitemap = False

    for _status, changed_paths in changes:
        for changed_path in changed_paths:
            if changed_path == "CHANGELOG.md":
                public_paths.add("/changelog")
                continue

            content_path = _content_public_path(changed_path)
            if content_path:
                public_paths.add(content_path)
                if content_path.startswith("/blog/"):
                    public_paths.add("/blog")
                elif content_path.startswith("/use-cases/"):
                    public_paths.add("/use-cases")
            elif _is_global_public_change(changed_path):
                include_sitemap = True

    return public_paths, include_sitemap


def _normalized_site_url(site_url: str) -> str:
    normalized = site_url.strip().rstrip("/")
    parsed = urlsplit(normalized)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise IndexNowError("INDEXNOW_SITE_URL must be an absolute HTTP or HTTPS URL.")
    if parsed.path or parsed.query or parsed.fragment:
        raise IndexNowError("INDEXNOW_SITE_URL must not include a path, query, or fragment.")
    return normalized


def _same_host_urls(site_url: str, urls: Iterable[str]) -> list[str]:
    expected = urlsplit(site_url)
    validated = []
    seen = set()
    for url in urls:
        parsed = urlsplit(url)
        if parsed.scheme not in {"http", "https"} or parsed.netloc != expected.netloc:
            raise IndexNowError(f"IndexNow URL {url!r} does not belong to {expected.netloc}.")
        if url not in seen:
            seen.add(url)
            validated.append(url)
    return validated


def build_indexnow_payload(site_url: str, key: str, urls: Iterable[str]) -> dict:
    site_url = _normalized_site_url(site_url)
    if not is_valid_indexnow_key(key):
        raise IndexNowError("INDEXNOW_KEY must contain 8 to 128 ASCII letters, digits, or hyphens.")

    return {
        "host": urlsplit(site_url).netloc,
        "key": key,
        "keyLocation": f"{site_url}{INDEXNOW_KEY_PATH}",
        "urlList": _same_host_urls(site_url, urls),
    }


def _open(
    request: Request,
    *,
    urlopen: UrlOpen,
    attempts: int = 1,
    timeout: int = REQUEST_TIMEOUT_SECONDS,
    sleep: Callable[[float], None] = time.sleep,
):
    for attempt in range(1, attempts + 1):
        try:
            return urlopen(request, timeout=timeout)
        except HTTPError as exc:
            if exc.code not in RETRYABLE_HTTP_STATUSES or attempt == attempts:
                raise IndexNowError(f"IndexNow returned HTTP {exc.code}.") from exc
        except URLError as exc:
            if attempt == attempts:
                raise IndexNowError(f"IndexNow request failed: {exc.reason}.") from exc
        sleep(REQUEST_RETRY_DELAY_SECONDS * attempt)
    raise AssertionError("IndexNow retry loop ended unexpectedly.")


def fetch_sitemap_urls(
    site_url: str,
    *,
    attempts: int = 1,
    urlopen: UrlOpen = stdlib_urlopen,
) -> list[str]:
    site_url = _normalized_site_url(site_url)
    pending = [f"{site_url}/sitemap.xml"]
    visited = set()
    page_urls = []

    while pending:
        sitemap_url = pending.pop()
        if sitemap_url in visited:
            continue
        if len(visited) >= MAX_SITEMAP_DOCUMENTS:
            raise IndexNowError("Production sitemap index exceeds the supported document limit.")
        visited.add(sitemap_url)

        request = Request(
            sitemap_url,
            headers={"User-Agent": "rowset-indexnow-deploy/1.0"},
        )
        try:
            with _open(request, urlopen=urlopen, attempts=attempts) as response:
                document = ElementTree.fromstring(response.read())
        except ElementTree.ParseError as exc:
            raise IndexNowError("Production sitemap returned invalid XML.") from exc

        locations = [
            element.text.strip() for element in document.findall(".//{*}loc") if element.text
        ]
        root_tag = document.tag.rsplit("}", maxsplit=1)[-1]
        if root_tag == "sitemapindex":
            pending.extend(_same_host_urls(site_url, locations))
        elif root_tag == "urlset":
            page_urls.extend(_same_host_urls(site_url, locations))
        else:
            raise IndexNowError(f"Production sitemap has unsupported root element {root_tag!r}.")

    return _same_host_urls(site_url, page_urls)


def verify_key_location(
    site_url: str,
    key: str,
    *,
    attempts: int = 1,
    urlopen: UrlOpen = stdlib_urlopen,
) -> None:
    site_url = _normalized_site_url(site_url)
    request = Request(
        f"{site_url}{INDEXNOW_KEY_PATH}",
        headers={"User-Agent": "rowset-indexnow-deploy/1.0"},
    )
    with _open(request, urlopen=urlopen, attempts=attempts) as response:
        deployed_key = response.read().decode("utf-8").strip()
    if not hmac.compare_digest(deployed_key, key):
        raise IndexNowError("The deployed IndexNow key file does not match INDEXNOW_KEY.")


def submit_urls(
    site_url: str,
    key: str,
    urls: Iterable[str],
    *,
    attempts: int = 1,
    sleep: Callable[[float], None] = time.sleep,
    urlopen: UrlOpen = stdlib_urlopen,
) -> int:
    site_url = _normalized_site_url(site_url)
    validated_urls = _same_host_urls(site_url, urls)
    submitted = 0

    for start in range(0, len(validated_urls), MAX_URLS_PER_REQUEST):
        batch = validated_urls[start : start + MAX_URLS_PER_REQUEST]
        payload = build_indexnow_payload(site_url, key, batch)
        request = Request(
            INDEXNOW_ENDPOINT,
            data=json.dumps(payload, separators=(",", ":")).encode("utf-8"),
            headers={
                "Content-Type": "application/json; charset=utf-8",
                "User-Agent": "rowset-indexnow-deploy/1.0",
            },
            method="POST",
        )
        with _open(
            request,
            urlopen=urlopen,
            attempts=attempts,
            sleep=sleep,
        ) as response:
            if response.status not in {200, 202}:
                raise IndexNowError(f"IndexNow returned unexpected HTTP {response.status}.")
        submitted += len(batch)

    return submitted


def _absolute_urls(site_url: str, paths: Iterable[str]) -> list[str]:
    base_url = f"{_normalized_site_url(site_url)}/"
    return [urljoin(base_url, path.lstrip("/")) for path in sorted(paths)]


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Submit deployed public URL changes to IndexNow.")
    parser.add_argument("--site-url", required=True)
    parser.add_argument("--before", required=True)
    parser.add_argument("--after", required=True)
    args = parser.parse_args(argv)
    key = os.environ.get("INDEXNOW_KEY", "").strip()

    if not key:
        print("INDEXNOW_KEY is not configured; skipping IndexNow notification.")
        return 0
    if not is_valid_indexnow_key(key):
        raise IndexNowError("INDEXNOW_KEY must contain 8 to 128 ASCII letters, digits, or hyphens.")

    site_url = _normalized_site_url(args.site_url)
    if args.before == ZERO_SHA:
        public_paths, include_sitemap = set(), True
    else:
        public_paths, include_sitemap = public_paths_for_changes(
            read_git_changes(args.before, args.after)
        )

    urls = _absolute_urls(site_url, public_paths)
    if include_sitemap:
        urls.extend(fetch_sitemap_urls(site_url, attempts=REQUEST_RETRY_ATTEMPTS))
    urls = _same_host_urls(site_url, urls)

    if not urls:
        print("No public URL changes detected; skipping IndexNow notification.")
        return 0

    verify_key_location(site_url, key, attempts=REQUEST_RETRY_ATTEMPTS)
    submitted = submit_urls(site_url, key, urls, attempts=REQUEST_RETRY_ATTEMPTS)
    print(f"Submitted {submitted} changed public URL(s) to IndexNow.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except IndexNowError as exc:
        print(f"IndexNow notification failed: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
