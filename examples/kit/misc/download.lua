return tools.define_tool({
    source = "git@github.com:tchaudhry91/python-html-downloader.git",
    entrypoint = "download.py",
    type = "python",
    input_adapter = "json",
    output_adapter = "json",
    schema = {
        input = {
            url = "string",
        },
        output = {
            html = "string",
        },
    },
})
