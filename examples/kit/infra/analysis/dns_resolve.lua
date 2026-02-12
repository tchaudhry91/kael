return tools.define_tool({
    source = "/home/tchaudhry/Workspace/kael/examples/scripts/analysis",
    entrypoint = "dns_resolve.js",
    type = "node",
    input_adapter = "json",
    output_adapter = "json",
    schema = {
        input = {
            domain = "string",
            record_types = "array?",
        },
        output = {
            domain = "string",
            records = "object",
            reverse = "array?",
        },
    },
})
