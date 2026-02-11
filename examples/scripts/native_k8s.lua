local output = kit.kubernetes.node_capacity({})

for i, node in ipairs(output.nodes) do
	print("Node:" .. node.name .. "\tCPU:" .. node.capacity.cpu .. "\tMemory:" .. node.capacity.memory)
end

local pending_pods = kit.kubernetes.pending_pods({})

print("Pending pods: " .. pending_pods.count)
for i, pod in ipairs(pending_pods.pods) do
	print("  " .. pod.namespace .. "/" .. pod.name)
end
