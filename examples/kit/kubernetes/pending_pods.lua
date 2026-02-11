local base = require("kubernetes.defaults")()
base.entrypoint = "pending_pods.py"
return tools.define_tool(base)
