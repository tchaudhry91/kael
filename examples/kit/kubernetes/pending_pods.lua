local base = require("kubernetes.defaults")()
base.entrypoint = "pending_pods.py"
base.type = "python"
return tools.define_tool(base)
