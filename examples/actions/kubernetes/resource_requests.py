import json
import re
import subprocess
import sys


def parse_cpu(val):
    if not val:
        return 0.0
    if val.endswith("m"):
        return float(val[:-1]) / 1000.0
    return float(val)


def parse_memory_mi(val):
    if not val:
        return 0.0
    units = {"Ki": 1 / 1024, "Mi": 1, "Gi": 1024, "Ti": 1024 * 1024}
    for suffix, factor in units.items():
        if val.endswith(suffix):
            return float(val[: -len(suffix)]) * factor
    return float(val) / (1024 * 1024)


def main():
    params = json.load(sys.stdin)
    namespace = params.get("namespace", "default")

    cmd = ["kubectl", "get", "pods", "-o", "json"]
    if namespace == "all":
        cmd.append("--all-namespaces")
    else:
        cmd.extend(["-n", namespace])

    result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    data = json.loads(result.stdout)

    total_req_cpu = 0.0
    total_req_mem = 0.0
    total_lim_cpu = 0.0
    total_lim_mem = 0.0

    pod_details = []
    for item in data.get("items", []):
        if item["status"].get("phase") not in ("Running", "Pending"):
            continue

        pod_req_cpu = 0.0
        pod_req_mem = 0.0
        pod_lim_cpu = 0.0
        pod_lim_mem = 0.0

        for c in item["spec"].get("containers", []):
            res = c.get("resources", {})
            req = res.get("requests", {})
            lim = res.get("limits", {})
            pod_req_cpu += parse_cpu(req.get("cpu", ""))
            pod_req_mem += parse_memory_mi(req.get("memory", ""))
            pod_lim_cpu += parse_cpu(lim.get("cpu", ""))
            pod_lim_mem += parse_memory_mi(lim.get("memory", ""))

        total_req_cpu += pod_req_cpu
        total_req_mem += pod_req_mem
        total_lim_cpu += pod_lim_cpu
        total_lim_mem += pod_lim_mem

        pod_details.append({
            "name": item["metadata"]["name"],
            "namespace": item["metadata"]["namespace"],
            "requests": {"cpu": round(pod_req_cpu, 3), "memory_mi": round(pod_req_mem, 1)},
            "limits": {"cpu": round(pod_lim_cpu, 3), "memory_mi": round(pod_lim_mem, 1)},
        })

    json.dump({
        "totals": {
            "requests": {"cpu": round(total_req_cpu, 3), "memory_mi": round(total_req_mem, 1)},
            "limits": {"cpu": round(total_lim_cpu, 3), "memory_mi": round(total_lim_mem, 1)},
        },
        "pods": pod_details,
    }, sys.stdout)


if __name__ == "__main__":
    main()
