#!/usr/bin/env python3
"""Probe a list of URLs and return health info: status, latency, redirects, TLS cert expiry."""
import json
import socket
import ssl
import sys
from datetime import datetime, timezone
from urllib.parse import urlparse

import requests


def get_cert_expiry(hostname, port=443, timeout=5):
    """Connect via TLS and return cert expiry as ISO string and days remaining."""
    try:
        ctx = ssl.create_default_context()
        with socket.create_connection((hostname, port), timeout=timeout) as sock:
            with ctx.wrap_socket(sock, server_hostname=hostname) as ssock:
                cert = ssock.getpeercert()
                expiry = datetime.strptime(cert["notAfter"], "%b %d %H:%M:%S %Y %Z")
                expiry = expiry.replace(tzinfo=timezone.utc)
                days = (expiry - datetime.now(timezone.utc)).days
                return {"expires": expiry.isoformat(), "days_remaining": days}
    except Exception:
        return None


def probe_url(url, timeout=10):
    """Probe a single URL and return structured result."""
    result = {"url": url}
    try:
        resp = requests.get(url, timeout=timeout, allow_redirects=True)
        result["status"] = resp.status_code
        result["latency_ms"] = round(resp.elapsed.total_seconds() * 1000)
        result["content_length"] = len(resp.content)

        if resp.history:
            result["redirects"] = [
                {"url": r.url, "status": r.status_code} for r in resp.history
            ]

        parsed = urlparse(url)
        if parsed.scheme == "https":
            cert = get_cert_expiry(parsed.hostname)
            if cert:
                result["tls"] = cert

    except requests.exceptions.Timeout:
        result["error"] = "timeout"
    except requests.exceptions.ConnectionError as e:
        result["error"] = f"connection_error: {e}"
    except Exception as e:
        result["error"] = str(e)

    return result


def main():
    params = json.load(sys.stdin)
    urls = params.get("urls", [])
    timeout = params.get("timeout", 10)

    if isinstance(urls, str):
        urls = [urls]

    results = [probe_url(url, timeout=timeout) for url in urls]
    json.dump({"results": results}, sys.stdout)


if __name__ == "__main__":
    main()
