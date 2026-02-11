local base = require("kubernetes.defaults")()
base.entrypoint = "restart_counts.py"
return tools.define_tool(base)
