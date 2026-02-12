local base = require("infra.kubernetes.defaults")()
base.entrypoint = "binpacking_report.py"
base.type = "python"
base.output_adapter = "json"
base.schema = {
    input = {
        node = "string?",
    },
    output = {
        nodes = "array",
    },
}
return tools.define_tool(base)
