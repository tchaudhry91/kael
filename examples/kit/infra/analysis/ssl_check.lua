return tools.define_tool({
    source = "/home/tchaudhry/Workspace/kael/examples/scripts/analysis",
    entrypoint = "ssl_check.py",
    type = "python",
    deps = {"certifi"},
    input_adapter = "json",
    output_adapter = "json",
    schema = {
        input = {
            hostnames = "array",
            port = "number?",
            timeout = "number?",
        },
        output = {
            results = "array",
        },
    },
})
