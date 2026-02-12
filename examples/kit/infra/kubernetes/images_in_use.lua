local base = require("infra.kubernetes.defaults")()
base.entrypoint = "images_in_use.sh"
base.type = "shell"
base.input_adapter = "json"
base.output_adapter = "json"
base.schema = {
    input = {
        namespace = "string?",
    },
    output = {
        images = "array",
    },
}
return tools.define_tool(base)
