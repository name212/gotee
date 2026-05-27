include makefile.inc/bin.mk makefile.inc/duration.mk makefile.inc/golang.mk 

all: go/lint go/test go/test/race

test: go/test