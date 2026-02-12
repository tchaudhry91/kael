local base = require("kubernetes.defaults")()
base.entrypoint = "images_in_use.sh"
base.type = "shell"
base.schema = {
	input = {
		namespace = "string?",
	},
	output = {
		images = "array",
		unique_images = "array",
	},
}
return tools.define_tool(base)
