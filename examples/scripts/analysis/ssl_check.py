#!/usr/bin/env python3
"""Check SSL/TLS certificate details for one or more hostnames."""
import json
import socket
import ssl
import sys
from datetime import datetime, timezone

import certifi


def check_host(hostname, port=443, timeout=5):
    """Connect to a host and return certificate details."""
    result = {"hostname": hostname, "port": port}
    try:
        ctx = ssl.create_default_context(cafile=certifi.where())
        with socket.create_connection((hostname, port), timeout=timeout) as sock:
            with ctx.wrap_socket(sock, server_hostname=hostname) as ssock:
                cert = ssock.getpeercert()
                result["protocol"] = ssock.version()
                result["cipher"] = ssock.cipher()[0]

                # Parse dates
                not_before = datetime.strptime(cert["notBefore"], "%b %d %H:%M:%S %Y %Z")
                not_after = datetime.strptime(cert["notAfter"], "%b %d %H:%M:%S %Y %Z")
                not_after = not_after.replace(tzinfo=timezone.utc)
                now = datetime.now(timezone.utc)

                result["issuer"] = dict(x[0] for x in cert.get("issuer", []))
                result["subject"] = dict(x[0] for x in cert.get("subject", []))
                result["not_before"] = not_before.isoformat()
                result["not_after"] = not_after.isoformat()
                result["days_remaining"] = (not_after - now).days
                result["expired"] = not_after < now

                # SANs
                sans = []
                for type_val in cert.get("subjectAltName", []):
                    sans.append({"type": type_val[0], "value": type_val[1]})
                result["san"] = sans

    except ssl.SSLCertVerificationError as e:
        result["error"] = f"certificate verification failed: {e}"
    except socket.timeout:
        result["error"] = "connection timed out"
    except ConnectionRefusedError:
        result["error"] = "connection refused"
    except Exception as e:
        result["error"] = str(e)

    return result


def main():
    params = json.load(sys.stdin)
    hostnames = params.get("hostnames", [])
    port = params.get("port", 443)
    timeout = params.get("timeout", 5)

    if isinstance(hostnames, str):
        hostnames = [hostnames]

    results = [check_host(h, port=port, timeout=timeout) for h in hostnames]
    json.dump({"results": results}, sys.stdout)


if __name__ == "__main__":
    main()
