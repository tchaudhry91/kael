#!/usr/bin/env bash
set -euo pipefail

input=$(cat)
namespace=$(echo "$input" | jq -r '.namespace // empty')

if [ -n "$namespace" ] && [ "$namespace" != "all" ]; then
    data=$(kubectl get pods -n "$namespace" -o json)
else
    data=$(kubectl get pods --all-namespaces -o json)
fi

echo "$data" | jq '{images: [.items[] | {
    namespace: .metadata.namespace,
    pod: .metadata.name,
    containers: [.spec.containers[] | {name: .name, image: .image}]
}], unique_images: ([.items[].spec.containers[].image] | unique)}'
