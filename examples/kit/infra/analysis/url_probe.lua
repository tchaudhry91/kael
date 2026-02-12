return tools.define_tool({
    source = "/home/tchaudhry/Workspace/kael/examples/scripts/analysis",
    entrypoint = "url_probe.py",
    type = "python",
    deps = {"requests"},
    input_adapter = "json",
    output_adapter = "json",
    schema = {
        input = {
            urls = "array",
            timeout = "number?",
        },
        output = {
            results = "array",
        },
    },
})
