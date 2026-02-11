local base = require("kubernetes.defaults")()
base.entrypoint = "pods_on_node.py"
base.type = "python"
return tools.define_tool(base)
