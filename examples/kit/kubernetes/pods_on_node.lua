local base = require("kubernetes.defaults")()
base.entrypoint = "pods_on_node.py"
return tools.define_tool(base)
