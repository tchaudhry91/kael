local base = require("kubernetes.defaults")()
base.entrypoint = "images_in_use.sh"
return tools.define_tool(base)
