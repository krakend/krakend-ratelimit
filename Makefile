benchmark:
	clear
	go test -bench=. -count 5 -benchmem -run=^#
.PHONY: benchmark
