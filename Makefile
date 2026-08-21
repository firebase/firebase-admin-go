unit:
	go test -v -race -test.short ./...

lint:
	go get -u golang.org/x/lint/golint
	`go list -f {{.Target}} golang.org/x/lint/golint` -set_exit_status ./...

format:
	gofmt -d -s .

analysis:
	go vet -v ./...

all: format lint analysis unit
