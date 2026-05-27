
GOLANGCI_VERSION  = 2.11.4
GOFUMPT_VERSION   = 0.9.2
JQ_VERSION_FOR_GO = 1.8.1

ifeq ($(OS_NAME), Linux)
	JQ_PLATFORM_FOR_GO = linux
	GOFUMPT_PLATFORM = linux
else ifeq ($(OS_NAME), Darwin)
	JQ_PLATFORM_FOR_GO = macos
	GOFUMPT_PLATFORM = darwin
endif

ifeq ($(PLATFORM_NAME), x86_64)
	JQ_PLATFORM_ARCH_FOR_GO = $(JQ_PLATFORM_FOR_GO)-amd64
	GOFUMPT_ARCH = amd64
else ifeq ($(PLATFORM_NAME), arm64)
	JQ_PLATFORM_ARCH_FOR_GO = $(JQ_PLATFORM_FOR_GO)-arm64
	GOFUMPT_ARCH = arm64
endif

tmp/go-tests:
	@mkdir -p tmp/go-tests

check/installed/go:
	@command -v go > /dev/null
	@go version

install/go/jq: bin check/installed/curl
	$(shell $(call CHECK_BINARY,jq-for-go,--version,$(JQ_VERSION_FOR_GO)))
	@if [ "$(.SHELLSTATUS)" -ne 0 ]; then \
		echo "Install jq for go"; \
		curl -sSfL https://github.com/jqlang/jq/releases/download/jq-$(JQ_VERSION_FOR_GO)/jq-$(JQ_PLATFORM_ARCH_FOR_GO) -o ./bin/jq-for-go; \
		chmod +x "./bin/jq-for-go"; \
	fi

install/go/go-fumpt: bin check/installed/curl
	$(shell $(call CHECK_BINARY,gofumpt,-version,$(GOFUMPT_VERSION)))
	@if [ "$(.SHELLSTATUS)" -ne 0 ]; then \
		echo "Install gofumpt"; \
		curl -sSfLo "bin/gofumpt" https://github.com/mvdan/gofumpt/releases/download/v$(GOFUMPT_VERSION)/gofumpt_v$(GOFUMPT_VERSION)_$(GOFUMPT_PLATFORM)_$(GOFUMPT_ARCH); \
		chmod +x "./bin/gofumpt"; \
	fi


install/go/golangci-lint: bin check/installed/curl
	$(shell $(call CHECK_BINARY,golangci-lint,--version,$(GOLANGCI_VERSION)))
	@if [ "$(.SHELLSTATUS)" -ne 0 ]; then \
		echo "Install golangci-lint"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | BINARY=golangci-lint bash -s -- v${GOLANGCI_VERSION}; \
		chmod +x "./bin/golangci-lint"; \
	fi

install/go/deps/lint: install/go/go-fumpt install/go/golangci-lint
install/go/deps: install/go/deps/lint install/go/jq

go/lint: install/go/deps/lint check/installed/go
	@if [ ! -f .golangci.yaml ]; then \
		echo -e "${RED_COLOR}.golangci.yaml is not present!${NO_COLOR}"; \
		exit 1; \
	fi;\
	fix_arg=""; \
	if [ -n "$$RUN_FIX" ]; then \
		fix_arg="--fix"; \
	fi; \
	full_cfg="$$(realpath .golangci.yaml)"; \
	full_lint_bin="$$(realpath bin/golangci-lint)"; \
	failed=""; \
	for ii in $$(find . -type f -name "go.mod" -printf "%h\n" | sort -u); do \
		pushd . > /dev/null; \
		full_path="$$(realpath "$$ii")"; \
		echo "--- Run lint in $$full_path ---"; \
		if ! cd "$$full_path"; then \
			echo -e "${RED_COLOR}Cannot cd to $$full_path${NO_COLOR}"; \
			exit 1; \
		fi; \
		if ! "$$full_lint_bin" run ./... -c "$$full_cfg" $$fix_arg; then \
			failed="true"; \
		fi; \
		popd > /dev/null; \
	done; \
	if [ -n "$$failed" ]; then \
		echo -e "${RED_COLOR}Code is not linted!${NO_COLOR}"; \
		exit 1; \
	fi

go/lint/fix: export RUN_FIX = true
go/lint/fix: go/lint

go/tidy: check/installed/go
	@for ii in $$(find . -type f -name "go.mod" -printf "%h\n" | sort -u); do \
		pushd . > /dev/null; \
		full_path="$$(realpath "$$ii")"; \
		echo "Run go mod tidy in $$full_path"; \
		go mod tidy; \
		popd > /dev/null; \
	done

go/ci/check/no-tidy: go/tidy
	@if ! git diff --exit-code; then \
		echo ""; \
		echo -e "${RED_COLOR}go mod tidy produce diff. Please run 'make go/tidy' and commit${NO_COLOR}"; \
		exit 1; \
	fi

go/test: check/installed/go install/go/jq tmp/go-tests
	@race_arg=""; \
	if [ -n "$$RUN_RACE" ]; then \
		race_arg="-race"; \
	fi; \
	force_restart_arg=""; \
	if [ -n "$$FORCE_RESTART" ]; then \
		force_restart_arg="-count=1"; \
	fi; \
	root_dir="$$(realpath .)"; \
	jq_bin="$${root_dir}/bin/jq-for-go"; \
	failed=""; \
	failed_files=(); \
	start_micro="$$(date +"%s%6N")"; \
	for ii in $$(find . -type f -name "go.mod" -printf "%h\n" | sort -u); do \
		out_file="$${root_dir}/tmp/go-tests/$$(date +%F_%H-%M-%S).json.test.result"; \
		pushd . > /dev/null; \
		full_path="$$(realpath "$$ii")"; \
		echo "--- Run tests in $$full_path ---"; \
		if ! cd "$$full_path"; then \
			echo -e "${RED_COLOR}Cannot cd to $$full_path${NO_COLOR}"; \
			exit 1; \
		fi; \
		go test $$force_restart_arg $$race_arg -json -v -p 1 ./... | tee "$$out_file" | "$$jq_bin" -r 'if has("Output") then .Output else "" end' | grep -v '^$$'; \
		if [ "$${PIPESTATUS[0]}" != "0" ]; then \
			failed_files+=("$$out_file"); \
			failed="true"; \
		else \
			rm -f "$$out_file"; \
		fi; \
		popd > /dev/null; \
	done; \
	end_micro="$$(date +"%s%6N")"; \
	failed_tests=""; \
	for fail_file in "$${failed_files[@]}"; do \
		fail_for_file="$$("$$jq_bin" -r 'select(.Action == "fail" and has("Test")) | .Test' "$$fail_file")"; \
		if [ -z "$$fail_for_file" ]; then \
			continue; \
		fi; \
		IFS=$$'\n' read -rd '' -a list_failed <<< "$$fail_for_file"; \
		for fail_test_name in "$${list_failed[@]}"; do \
			failed_tests+=$$'\n'; \
			failed_tests+="\"$$fail_test_name\""; \
			echo -e "${RED_COLOR}--- Unsuccessful test $$fail_test_name ---${NO_COLOR}"; \
			echo -e "${RED_COLOR}$$("$$jq_bin" -r "select(has(\"Test\") and has(\"Output\") and .Test == \"$$fail_test_name\") | .Output" "$$fail_file" | grep -v '^$$')${NO_COLOR}"; \
		done; \
		rm -f "$$fail_file"; \
	done; \
	if [ -n "$$failed" ]; then \
		echo "";\
		echo -e "${RED_COLOR}Unsuccessful tests:${NO_COLOR}"; \
		echo -e "${RED_COLOR}$$("$$jq_bin" -rs "flatten | sort_by(. | split(\"/\") | length - 1) | join(\"\n\")"<<<"$$failed_tests")${NO_COLOR}"; \
		echo "";\
		exit 1; \
	fi; \
	tests_duration="$$(${HUMAN_DURATION_MICROSECONDS} "dummy" "$$start_micro" "$$end_micro")"; \
	echo -e "${GREEN_COLOR}Tests passed in $$tests_duration!${NO_COLOR}"

go/test/race: export RUN_RACE = true
go/test/race: export FORCE_RESTART = true
go/test/race: go/test

go/test/force: export FORCE_RESTART = true
go/test/force: go/test

clean/golang:
	@rm -f bin/gofumpt
	@rm -f bin/golangci-lint
	@rm -f bin/jq-for-go