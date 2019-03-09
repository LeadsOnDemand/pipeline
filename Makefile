clean:
	rm trace/main
	rm trace/m.trace
build:
	go build -o trace/main trace/main.go
tracer: build
	trace/main > trace/m.trace
	go tool trace trace/m.trace
test:
	go test
time: build
	time ./trace/main > /dev/null