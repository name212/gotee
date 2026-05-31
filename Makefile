include makefile-go.inc/makefile.inc/versions.mk makefile-go.inc/makefile.inc/duration.mk makefile-go.inc/makefile.inc/bin.mk makefile-go.inc/makefile.inc/golang.mk

all: go/lint go/test go/test/race

test: go/test