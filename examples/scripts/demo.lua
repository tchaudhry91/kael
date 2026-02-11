local output = kit.kubernetes.node_capacity({})

for i, node in ipairs(output.nodes) do
	print("Node:" .. node.name .. "\tCPU:" .. node.capacity.cpu .. "\tMemory:" .. node.capacity.memory)
end
