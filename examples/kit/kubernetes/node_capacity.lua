local base = require("kubernetes.defaults")()
base.entrypoint = "node_capacity.sh"
base.type = "shell"
return tools.define_tool(base)
