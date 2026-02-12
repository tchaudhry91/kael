local opts = {
	source = "git@github.com:tchaudhry91/python-html-downloader",
	schema = {
		input = {
			url = "string",
		},
		output = {
			content = "string",
		},
	},
}

return tools.define_tool(opts)
