gen:
	buf dep update
	buf format -w
	buf generate

logfile1 := ./temp/part1/rlog
test1:
	@([ -d "temp" ] && rm -rf temp/* || true)
	mkdir -p ./temp/part1
	@go clean -testcache
	go test ./part1 -v -race 2>&1 | tee "$(logfile1)"
	go run tools/main.go -part 1 < "$(logfile1)"

