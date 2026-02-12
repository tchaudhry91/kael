local base = require("kubernetes.defaults")()
base.entrypoint = "restart_counts.py"
base.type = "python"
base.schema = {
	input = {
		namespace = "string?",
		threshold = "number?",
	},
	output = {
		namespace = "string",
		threshold = "number",
		pods = "array",
	},
}
return tools.define_tool(base)
