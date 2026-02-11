local base = require("kubernetes.defaults")()
base.entrypoint = "images_in_use.sh"
base.type = "shell"
return tools.define_tool(base)
