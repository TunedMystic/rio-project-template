APP="$$(basename -- $$(PWD))"



# -------------------------------------------------------------------
# App-related commands
# -------------------------------------------------------------------

## @(app) - 🤖 Run the Go app       (dev)
run: bin/watchexec bin/cwebp
	@echo "✨🤖✨ Running the app server\n"
	@./bin/watchexec -r -e go,css,js,html,md,json "go run -ldflags=\"-X 'main.BuildEnv=debug'\" ./..."


## @(app) - 📦 Build the app binary
build: clean tailwind
	@echo "✨📦✨ Building the app binary\n"
	@go build -ldflags="-s -w -X 'main.BuildHash=$$(git rev-parse --short=10 HEAD)' -X 'main.BuildDate=$$(date)'" -o bin/app ./...


## @(app) - 🚀 Deploy the app with Fly.io
deploy: clean tailwind
	@echo "\n"
	@echo "✨🚀✨ Deploying application\n"
	@fly deploy --no-cache --build-arg BUILD_HASH="$$(git rev-parse --short=10 HEAD)"


## @(app) - ✨ Remove temp files and dirs
clean:
	@echo "✨✨ Cleaning temp files\n"
	@rm -f coverage.out
	@go clean -testcache
	@find . -name '.DS_Store' -type f -delete
	@bash -c "mkdir -p bin && cd bin && find . ! -name 'watchexec' ! -name 'cwebp' ! -name 'tailwind' -type f -exec rm -f {} +"



# -------------------------------------------------------------------
# Docker-related commands
# -------------------------------------------------------------------

## @(docker) - 🤖 Run the Docker image
run-docker: build-docker
	@echo "✨🤖✨ Running the docker image\n"
	docker run --rm --publish "3000:3000" --name ${APP} ${APP}


## @(docker) - 📦 Build the docker image
build-docker: clean tailwind
	@echo "✨📦✨ Building the docker image\n"
	@docker build --build-arg BUILD_HASH="$$(git rev-parse --short=10 HEAD)" -t ${APP} .


## @(docker) - 📦 Build the docker image, no-cache
build-docker-n: clean tailwind
	@echo "✨📦✨ Building the docker image\n"
	@docker build --no-cache --build-arg BUILD_HASH="$$(git rev-parse --short=10 HEAD)" -t ${APP} .


## @(docker) - 🔥 Destroy all containers
wipe:
	@echo "✨🔥✨ Destroying related containers"
	@docker container rm -fv $$(docker container ls -aq) 2> /dev/null || true


## @(docker) - 💥 Destroy all containers, images and volumes
wipeall: wipe
	@echo "✨💥✨ Destroying related images"
	@docker image rm -f $$(docker image ls -q "*/*/${APP}*") $$(docker image ls -q --filter dangling=true) 2> /dev/null || true



# -------------------------------------------------------------------
# Helper commands
# -------------------------------------------------------------------

tailwind: bin/tailwind
	@echo "✨📦✨ Running tailwind\n"
	@bash -c "./bin/tailwind --input ./tailwind.input.css --output ./static/css/styles.css --minify $(args)"


bin/tailwind:
	@echo "✨📦✨ Downloading tailwindcss binary\n"
	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64
	chmod +x tailwindcss-macos-arm64
	mkdir -p bin
	mv tailwindcss-macos-arm64 ./bin/tailwind
	@echo ""


bin/watchexec:
	@echo "✨📦✨ Downloading watchexec binary\n"
	curl -sL https://github.com/watchexec/watchexec/releases/download/v1.23.0/watchexec-1.23.0-aarch64-apple-darwin.tar.xz | tar -xz
	mkdir -p bin
	mv ./watchexec-1.23.0-aarch64-apple-darwin/watchexec ./bin/watchexec
	rm -rf watchexec-1.23.0-aarch64-apple-darwin
	@echo ""


bin/cwebp:
	@echo "✨📦✨ Downloading cwebp binary\n"
	curl -sL https://storage.googleapis.com/downloads.webmproject.org/releases/webp/libwebp-1.3.2-mac-arm64.tar.gz | tar -xz
	mkdir -p bin
	mv ./libwebp-1.3.2-mac-arm64/bin/cwebp ./bin/cwebp
	rm -rf libwebp-1.3.2-mac-arm64
	@echo ""



# -------------------------------------------------------------------
# Self-documenting Makefile targets - https://bit.ly/32lE64t
# -------------------------------------------------------------------

.DEFAULT_GOAL := help

help:
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@awk '/^[a-zA-Z\-\_0-9]+:/ \
		{ \
			helpMessage = match(lastLine, /^## (.*)/); \
			if (helpMessage) { \
				helpCommand = substr($$1, 0, index($$1, ":")-1); \
				helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
				helpGroup = match(helpMessage, /^@([^ ]*)/); \
				if (helpGroup) { \
					helpGroup = substr(helpMessage, RSTART + 1, index(helpMessage, " ")-2); \
					helpMessage = substr(helpMessage, index(helpMessage, " ")+1); \
				} \
				printf "%s|  %-20s %s\n", \
					helpGroup, helpCommand, helpMessage; \
			} \
		} \
		{ lastLine = $$0 }' \
		$(MAKEFILE_LIST) \
	| sort -t'|' -sk1,1 \
	| awk -F '|' ' \
			{ \
			cat = $$1; \
			if (cat != lastCat || lastCat == "") { \
				if ( cat == "0" ) { \
					print "\nTargets:" \
				} else { \
					gsub("_", " ", cat); \
					printf "\n%s\n", cat; \
				} \
			} \
			print $$2 \
		} \
		{ lastCat = $$1 }'
	@echo ""
