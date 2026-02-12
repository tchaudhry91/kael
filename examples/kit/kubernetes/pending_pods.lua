local base = require("kubernetes.defaults")()
base.entrypoint = "pending_pods.py"
base.type = "python"
base.schema = {
	input = {
		namespace = "string?",
	},
	output = {
		count = "number",
		pods = "array",
	},
}
return tools.define_tool(base)
