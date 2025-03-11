gen:
	buf dep update
	buf format -w
	buf generate

logfile1 := ./temp/part1/rlog
test1:
	@([ -d "temp/part1" ] && rm -rf temp/part1/* || true)
	mkdir -p ./temp/part1
	@go clean -testcache
	go test ./part1 -v -race 2>&1 | tee "$(logfile1)"
	go run tools/main.go -part 1 < "$(logfile1)"

logfile2 := ./temp/part2/rlog
test2:
	@([ -d "temp/part2" ] && rm -rf temp/part2/* || true)
	mkdir -p ./temp/part2
	@go clean -testcache
	go test ./part2 -v -race 2>&1 | tee "$(logfile2)"
	go run tools/main.go -part 2 < "$(logfile2)"
