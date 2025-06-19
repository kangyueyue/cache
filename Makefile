.PHONY: format,wire,test_store,test_etcd,run_demo,run_A,run_B,run_C,run-default,tidy

format:
	goimports -w .
	@echo "go format successfully..."

wire:
	cd cmd/app && wire
	@echo "wire successfully..."

test_store:
	@echo "Running test_store..."
	cd store && go test -v -cover

test_etcd:
	@echo "Running tests_etcd..."
	cd test && go test -v -cover

run_demo:
	@echo "Running demo...."
	go run cmd/demo.go

run_A:
	@echo "Running server A..."
	go run cmd/main.go --port=8080 --node=A

run_B:
	@echo "Running server B..."
	go run cmd/main.go --port=8081 --node=B

run_C:
	@echo "Running server C..."
	go run cmd/main.go --port=8082 --node=C

run-default:
	@echo "Running server with default values..."
	go run cmd/main.go

tidy:
	@echo "Running tidy..."
	go mod tidy




