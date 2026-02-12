package runtime

import (
	"fmt"
	"strings"
)

// generateDockerfile returns a Dockerfile for the given script type, entrypoint, and deps.
func generateDockerfile(scriptType, entrypoint string, deps []string) string {
	switch strings.ToLower(scriptType) {
	case "python":
		return pythonDockerfile(entrypoint, deps)
	case "node":
		return nodeDockerfile(entrypoint, deps)
	case "shell":
		return shellDockerfile(entrypoint, deps)
	default:
		return shellDockerfile(entrypoint, deps)
	}
}

func pythonDockerfile(entrypoint string, deps []string) string {
	pipInstall := ""
	if len(deps) > 0 {
		pipInstall = fmt.Sprintf("RUN pip install --no-cache-dir %s\n", strings.Join(deps, " "))
	}
	return fmt.Sprintf(`FROM python:3-alpine
RUN apk add --no-cache ca-certificates bash
WORKDIR /kael/app
COPY . .
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi
%sENTRYPOINT ["python", "%s"]
`, pipInstall, entrypoint)
}

func nodeDockerfile(entrypoint string, deps []string) string {
	npmInstall := ""
	if len(deps) > 0 {
		npmInstall = fmt.Sprintf("RUN npm install %s\n", strings.Join(deps, " "))
	}
	return fmt.Sprintf(`FROM node:lts-alpine
RUN apk add --no-cache ca-certificates bash
WORKDIR /kael/app
COPY . .
RUN if [ -f package.json ]; then npm install --production; fi
%sENTRYPOINT ["node", "%s"]
`, npmInstall, entrypoint)
}

func shellDockerfile(entrypoint string, deps []string) string {
	basePkgs := "ca-certificates bash curl jq"
	if len(deps) > 0 {
		basePkgs += " " + strings.Join(deps, " ")
	}
	return fmt.Sprintf(`FROM alpine:3
RUN apk add --no-cache %s
WORKDIR /kael/app
COPY . .
RUN chmod +x %s
ENTRYPOINT ["/bin/sh", "%s"]
`, basePkgs, entrypoint, entrypoint)
}

const dockerIgnore = `**/.git
**/.gitignore
**/node_modules
**/__pycache__
*.pyc
.kael/
`
