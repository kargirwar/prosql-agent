# prosql-agent
This is the desktop agent required for prosql.io
# Building
Mac: go build -ldflags="-X 'main.VERSION=0.6.4' -X 'main.ALLOW=https://prosql.io' -X 'main.OS=mac'"

Windows: go build -ldflags="-X 'main.VERSION=0.6.4' -X 'main.ALLOW=https://prosql.io' -X 'main.OS=windows'"

Linux: go build -ldflags="-X 'main.VERSION=0.6.4' -X 'main.ALLOW=https://prosql.io' -X 'main.OS=linux'"

Cross compile for other OSes:

Example:
GOOS=linux go build -ldflags="-X 'main.VERSION=0.6.4' -X 'main.ALLOW=https://prosql.io' -X 'main.OS=linux'"

# Installation
You can simply add the built executable to your startup programs. If you want a programmatic 
way to do it please refer https://github.com/kargirwar/prosqlctl
