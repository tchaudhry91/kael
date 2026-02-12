#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys


def kubectl_json(args):
    result = subprocess.run(
        ["kubectl"] + args + ["-o", "json"],
        capture_output=True, text=True
    )
    if result.returncode != 0:
        print(json.dumps({"error": result.stderr.strip()}), file=sys.stdout)
        sys.exit(1)
    return json.loads(result.stdout)


def parse_resource(s):
    """Parse a k8s resource quantity string to a float (base unit: cores / bytes)."""
    if not s:
        return 0.0
    s = str(s)
    if s.endswith("m"):
        return float(s[:-1]) / 1000.0
    if s.endswith("Ki"):
        return float(s[:-2]) * 1024
    if s.endswith("Mi"):
        return float(s[:-2]) * 1024 * 1024
    if s.endswith("Gi"):
        return float(s[:-2]) * 1024 * 1024 * 1024
    if s.endswith("Ti"):
        return float(s[:-2]) * 1024 * 1024 * 1024 * 1024
    return float(s)


def format_mem(b):
    """Format bytes as human-readable."""
    if b >= 1024 ** 3:
        return f"{b / (1024**3):.1f}Gi"
    if b >= 1024 ** 2:
        return f"{b / (1024**2):.0f}Mi"
    return f"{b:.0f}"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--node", help="filter to a specific node")
    args = parser.parse_args()

    nodes_data = kubectl_json(["get", "nodes"])
    pods_data = kubectl_json(["get", "pods", "--all-namespaces", "--field-selector=status.phase=Running"])

    # Build capacity map
    capacity = {}
    for node in nodes_data["items"]:
        name = node["metadata"]["name"]
        cap = node["status"]["allocatable"]
        capacity[name] = {
            "cpu": parse_resource(cap.get("cpu", "0")),
            "memory": parse_resource(cap.get("memory", "0")),
        }

    # Aggregate requests per node
    requests = {name: {"cpu": 0.0, "memory": 0.0, "pods": 0} for name in capacity}
    for pod in pods_data["items"]:
        node_name = pod["spec"].get("nodeName")
        if not node_name or node_name not in requests:
            continue
        requests[node_name]["pods"] += 1
        for container in pod["spec"].get("containers", []):
            res = container.get("resources", {}).get("requests", {})
            requests[node_name]["cpu"] += parse_resource(res.get("cpu", "0"))
            requests[node_name]["memory"] += parse_resource(res.get("memory", "0"))

    # Build report
    report = []
    for name in sorted(capacity):
        if args.node and name != args.node:
            continue
        cap = capacity[name]
        req = requests[name]
        cpu_pct = (req["cpu"] / cap["cpu"] * 100) if cap["cpu"] > 0 else 0
        mem_pct = (req["memory"] / cap["memory"] * 100) if cap["memory"] > 0 else 0
        report.append({
            "node": name,
            "pods": req["pods"],
            "cpu_requested": round(req["cpu"], 2),
            "cpu_allocatable": round(cap["cpu"], 2),
            "cpu_percent": round(cpu_pct, 1),
            "memory_requested": format_mem(req["memory"]),
            "memory_allocatable": format_mem(cap["memory"]),
            "memory_percent": round(mem_pct, 1),
        })

    json.dump({"nodes": report}, sys.stdout)


if __name__ == "__main__":
    main()
