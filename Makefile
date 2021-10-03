.PHONY: example

## Run the example package
example:
	@go get -u github.com/oblq/workerful
	@cd ./example && go run ./main.go