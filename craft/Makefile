gen:
	buf dep update
	buf format -w
	buf generate

logfile := ./temp/rlog
test:
	@([ -d "temp" ] && rm -rf temp/* || true)
	mkdir -p ./temp
	@go clean -testcache
	go test -v -race 2>&1 | tee "$(logfile)"
	go run tools/main.go < "$(logfile)"
