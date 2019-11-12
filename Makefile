install-linter:
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $(go env GOPATH)/bin v1.21.0

lint:
	golint cluster
	golangci-lint run -E dupl -E bodyclose -E goimports -E prealloc -E unparam -E unconvert -E goconst -E misspell
