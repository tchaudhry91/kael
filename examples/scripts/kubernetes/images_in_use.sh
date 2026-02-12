#!/usr/bin/env bash
set -euo pipefail

input=$(cat)
namespace=$(echo "$input" | jq -r '.namespace // empty')

if [ -n "$namespace" ]; then
    pods=$(kubectl get pods -n "$namespace" -o json 2>/dev/null)
else
    pods=$(kubectl get pods --all-namespaces -o json 2>/dev/null)
fi

echo "$pods" | jq '{
  images: [
    .items[]
    | .spec.containers[].image as $img
    | {image: $img, namespace: .metadata.namespace, pod: .metadata.name}
  ]
  | group_by(.image)
  | map({
      image: .[0].image,
      count: length,
      namespaces: [.[].namespace] | unique
    })
  | sort_by(-.count)
}'
