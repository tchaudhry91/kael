import json
import subprocess
import sys


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

    pods = []
    for item in data.get("items", []):
        if item["status"].get("phase") != "Pending":
            continue

        reasons = []
        for cond in item["status"].get("conditions", []):
            if cond.get("status") == "False" and cond.get("reason"):
                reasons.append({
                    "type": cond["type"],
                    "reason": cond["reason"],
                    "message": cond.get("message", ""),
                })

        for cs in item["status"].get("containerStatuses", []):
            waiting = cs.get("state", {}).get("waiting", {})
            if waiting.get("reason"):
                reasons.append({
                    "type": "container:" + cs["name"],
                    "reason": waiting["reason"],
                    "message": waiting.get("message", ""),
                })

        pods.append({
            "name": item["metadata"]["name"],
            "namespace": item["metadata"]["namespace"],
            "reasons": reasons,
        })

    json.dump({"count": len(pods), "pods": pods}, sys.stdout)


if __name__ == "__main__":
    main()
