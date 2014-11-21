# Run tests

testnames=simple
tests=$(addprefix test_, $(testnames))
trash=.trash/

all: ucat

test: clean ucat ${tests}
	@echo ${tests}
	@echo "*** tests passed ***"

# not sue why this doesn't work:
# test_%: test_%.sh
test_simple: test_simple.sh
	mkdir -p ${trash}
	@echo "*** running $@ ***"
	./$@.sh

clean:
	@echo "*** $@ ***"
	-rm -r ${trash}

deps: random ucat

ucat:
	go build

random:
	@echo "*** installing $@ ***"
	go get github.com/jbenet/go-random/random
	go build -o random github.com/jbenet/go-random/random

.PHONY: clean ucat ${tests}
