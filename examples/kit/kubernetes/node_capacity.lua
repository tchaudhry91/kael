local base = require("kubernetes.defaults")()
base.entrypoint = "node_capacity.sh"
base.type = "shell"
base.schema = {
	input = {
		node = "string?",
	},
	output = {
		nodes = "array",
	},
}
return tools.define_tool(base)
