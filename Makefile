.PHONY: format,wire,test_store,test_etcd,run_demo,run_A,run_B,run_C,run-default,tidy

road:
	go get -u github.com/kangyueyue/road@v1.0.0

format:
	goimports -w .
	@echo "go format successfully..."

wire:
	cd cmd/app && wire
	@echo "wire successfully..."

test_store:
	cd store && go test -v -cover
	@echo "test_store successfully..."

test_etcd:
	cd test && go test -v -cover
	@echo "tests_etcd successfully..."

run_demo:
	go run cmd/demo.go
	@echo "demo successfully...."

run_A:
	go run cmd/main.go --port=8080 --node=A
	@echo "server A.successfully.."

run_B:
	go run cmd/main.go --port=8081 --node=B
	@echo "server B successfully..."

run_C:
	go run cmd/main.go --port=8082 --node=C
	@echo "server C successfully..."

run-default:
	go run cmd/main.go
	@echo "server with default values successfully..."

tidy:
	go mod tidy
	@echo "tidy successfully..."




