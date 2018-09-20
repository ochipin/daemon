all:
	rm -rf test
	go test -v -cover -coverprofile cover.out
	go tool cover -func=cover.out

html:
	rm -rf test
	go test -cover -coverprofile cover.out
	go tool cover -html=cover.out

clean:
	rm -rf test cover.out
