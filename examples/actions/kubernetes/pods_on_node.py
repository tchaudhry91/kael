import json
import subprocess
import sys


def main():
    params = json.load(sys.stdin)
    node = params["node"]
    namespace = params.get("namespace", "")

    cmd = [
        "kubectl", "get", "pods",
        "--field-selector", f"spec.nodeName={node}",
        "-o", "json",
    ]
    if namespace:
        cmd.extend(["-n", namespace])
    else:
        cmd.append("--all-namespaces")

    result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    data = json.loads(result.stdout)

    pods = []
    for item in data.get("items", []):
        status = item["status"]["phase"]
        restarts = sum(
            cs.get("restartCount", 0)
            for cs in item["status"].get("containerStatuses", [])
        )
        pods.append({
            "name": item["metadata"]["name"],
            "namespace": item["metadata"]["namespace"],
            "status": status,
            "restarts": restarts,
        })

    json.dump({"node": node, "count": len(pods), "pods": pods}, sys.stdout)


if __name__ == "__main__":
    main()
