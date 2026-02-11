local base = require("kubernetes.defaults")()
base.entrypoint = "resource_requests.py"
base.type = "python"
return tools.define_tool(base)
