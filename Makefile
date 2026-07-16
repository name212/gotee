include makefile-go/include.mk.inc

export GO_TEST_PARALLEL = 1

all: go/lint go/test go/test/race

test: go/test/force
