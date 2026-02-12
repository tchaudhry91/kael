local base = require("kubernetes.defaults")()
base.entrypoint = "pods_on_node.py"
base.type = "python"
base.schema = {
	input = {
		node = "string",
		namespace = "string?",
	},
	output = {
		node = "string",
		count = "number",
		pods = "array",
	},
}
return tools.define_tool(base)
