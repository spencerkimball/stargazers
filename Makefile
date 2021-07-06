.PHONY: help default clean stargazers

default: stargazers

clean:
	rm -f ./stargazers

.format.txt: *.go analyze/*.go cmd/*.go fetch/*.go
	gofmt -w .
	echo "done" > .format.txt

stargazers: .format.txt *.go analyze/*.go cmd/*.go fetch/*.go Makefile
	go build -mod=vendor -o stargazers .

help:
	@echo "Usage: make [stargazers]"
	@echo "     stargazers       Make the stargazers to ./stargazers"