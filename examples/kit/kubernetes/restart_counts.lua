local base = require("kubernetes.defaults")()
base.entrypoint = "restart_counts.py"
base.type = "python"
return tools.define_tool(base)
