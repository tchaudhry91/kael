#!/usr/bin/env bash
set -euo pipefail

input=$(cat)
node=$(echo "$input" | jq -r '.node // empty')

if [ -n "$node" ]; then
    data=$(kubectl get node "$node" -o json)
    echo "$data" | jq '{nodes: [{
        name: .metadata.name,
        capacity: {cpu: .status.capacity.cpu, memory: .status.capacity.memory, pods: .status.capacity.pods},
        allocatable: {cpu: .status.allocatable.cpu, memory: .status.allocatable.memory, pods: .status.allocatable.pods}
    }]}'
else
    data=$(kubectl get nodes -o json)
    echo "$data" | jq '{nodes: [.items[] | {
        name: .metadata.name,
        capacity: {cpu: .status.capacity.cpu, memory: .status.capacity.memory, pods: .status.capacity.pods},
        allocatable: {cpu: .status.allocatable.cpu, memory: .status.allocatable.memory, pods: .status.allocatable.pods}
    }]}'
fi
