import json
import subprocess
import sys


def main():
    params = json.load(sys.stdin)
    namespace = params.get("namespace", "default")
    threshold = params.get("threshold", 0)

    cmd = ["kubectl", "get", "pods", "-o", "json"]
    if namespace == "all":
        cmd.append("--all-namespaces")
    else:
        cmd.extend(["-n", namespace])

    result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    data = json.loads(result.stdout)

    pods = []
    for item in data.get("items", []):
        for cs in item["status"].get("containerStatuses", []):
            restarts = cs.get("restartCount", 0)
            if restarts > threshold:
                pods.append({
                    "pod": item["metadata"]["name"],
                    "container": cs["name"],
                    "restarts": restarts,
                    "ready": cs.get("ready", False),
                })

    pods.sort(key=lambda p: p["restarts"], reverse=True)
    json.dump({"namespace": namespace, "threshold": threshold, "pods": pods}, sys.stdout)


if __name__ == "__main__":
    main()
