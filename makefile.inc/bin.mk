SHELL = /usr/bin/env bash

export PATH := $(abspath ./bin):$(PATH)

RED_COLOR   := \033[0;31m
GREEN_COLOR := \033[0;32m
NO_COLOR    := \033[0m

PLATFORM_NAME := $(shell uname -m)

OS_NAME := $(shell uname)
ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif

define CHECK_BINARY
binary="$(1)"; \
version_arg="$(2)"; \
version="$(3)"; \
\
if [ -z "$$binary" ]; then \
  echo "binary not passed as first CHECK_BINARY define arg" >&2; \
  exit 1; \
fi; \
if [ -z "$$version_arg" ]; then \
  echo "binary version argument not passed as second CHECK_BINARY define arg" >&2; \
  exit 1; \
fi; \
if [ -z "$$version" ]; then \
  echo "target binary version not passed as third CHECK_BINARY define arg" >&2; \
  exit 1; \
fi; \
\
binary_full_path="$$(pwd)/bin/$${binary}"; \
\
if [ ! -x "$$binary_full_path" ]; then \
  echo "$$binary_full_path not exists or not executable" >&2; \
  exit 1; \
fi; \
\
got_bin_ver="$$("$$binary_full_path" "$$version_arg")"; \
\
if ! grep -q "$$version" <<<"$$got_bin_ver"; then \
  echo "Version of $$binary_full_path not match $$version Version is $$got_bin_ver" >&2; \
  exit 1; \
fi; \
exit 0
endef

bin:
	@mkdir -p bin

check/installed/curl:
	@command -v curl > /dev/null
