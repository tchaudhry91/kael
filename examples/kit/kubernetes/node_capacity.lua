local base = require("kubernetes.defaults")()
base.entrypoint = "node_capacity.sh"
return tools.define_tool(base)
