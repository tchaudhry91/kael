local base = require("kubernetes.defaults")()
base.entrypoint = "resource_requests.py"
base.type = "python"
base.schema = {
	input = {
		namespace = "string?",
	},
	output = {
		totals = "object",
		pods = "array",
	},
}
return tools.define_tool(base)
