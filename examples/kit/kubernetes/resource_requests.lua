local base = require("kubernetes.defaults")()
base.entrypoint = "resource_requests.py"
return tools.define_tool(base)
