export GO111MODULE=on
APP_VERSION=0.1.0
OUTPUT_BINARY=bin
BUILD_DIR=cmd/app
BUILD_ENVPARMS:=CGO_ENABLED=0

# install project dependencies
.PHONY: deps
deps:
	@echo 'install dependencies'
	go mod tidy -v

.PHONY: build-binary
build-binary:
	@echo 'build app $(APP_SERVICE)'
	$(shell $(BUILD_ENVPARMS) go build -buildvcs=false -ldflags="-X 'config.version=$(APP_VERSION)'" -o ./$(OUTPUT_BINARY) ./$(BUILD_DIR))

.PHONY:
build: build-binary

.PHONY:
build-create:
	@echo 'build docker image and run app'
	docker-compose up -d --build

.PHONY:
store-logs:
	@echo 'store logs in to a file'
	docker-compose logs -f > dev.log

.PHONY:
gen-proto:
	@echo 'generate proto'
	# You need to install protoc
	# MacOS: brew install protobuf
	# Linux: sudo apt-get install protobuf-compiler
	#go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28 && \
	#go install google.golang.org/api/cmd/protoc-gen-go-api@v1.2 && \
	cd ./api/ && \
	protoc --go_out=../ --go-grpc_out=../ proto/service.proto
