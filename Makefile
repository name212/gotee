include makefile-go/include.mk.inc

export GO_TEST_PARALLEL = 1

all: go/lint go/test go/test/race

test: go/test/force

export GO_TARGET_MODULE = $(CURDIR)/examples
export GO_TARGET = .

example/build: export PROJECT_NAME = example
example/build: go/build/current

example/build/tags: export PROJECT_NAME = example-tags
example/build/tags: export GO_BUILD_TAGS = first,second
example/build/tags: go/build/current

example/build/tags/one: export PROJECT_NAME = example-tags-one
example/build/tags/one: export GO_BUILD_TAGS = first
example/build/tags/one: go/build/current

define BUILD_VARIABLES_ALL
main.first=first set//||\
main.second=second
endef

example/build/vars: export PROJECT_NAME = example-vars
example/build/vars: export GO_BUILD_VARIABLES = ${BUILD_VARIABLES_ALL}
example/build/vars: go/build/current

define BUILD_VARIABLES_FIRST
main.first=first only set 
endef

example/build/vars/first: export PROJECT_NAME = example-vars-first
example/build/vars/first: export GO_BUILD_VARIABLES = ${BUILD_VARIABLES_FIRST}
example/build/vars/first: go/build/current

define BUILD_VARIABLES_SECOND
main.second= second only set
endef

example/build/vars/second: export PROJECT_NAME = example-vars-second
example/build/vars/second: export GO_BUILD_VARIABLES = ${BUILD_VARIABLES_SECOND}
example/build/vars/second: go/build/current

example/build/dynamic: export PROJECT_NAME = example-dynamic
example/build/dynamic: export GO_BUILD_TAGS = dynamic
example/build/dynamic: export GO_BUILD_DYNAMIC = true
example/build/dynamic: go/build/current

example/build/dyn-tag-vars: export PROJECT_NAME = example-dtv
example/build/dyn-tag-vars: export GO_BUILD_TAGS = dynamic,first,second
example/build/dyn-tag-vars: export GO_BUILD_VARIABLES = ${BUILD_VARIABLES_ALL}
example/build/dyn-tag-vars: export GO_BUILD_DYNAMIC = true
example/build/dyn-tag-vars: go/build/current