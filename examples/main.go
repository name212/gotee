package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/name212/gotee/pkg/tee/cmd"
	"github.com/name212/gotee/pkg/tee/consumer"
)

var (
	first  = "not set"
	second = "not set"
)

func main() {
	fmt.Printf("Build with '%s'\n", getTagStr())
	fmt.Printf("Variables:\n\tfirst='%s'\n\tsecond='%s'\n\n", first, second)
	fmt.Printf("getUser: '%s'\n", getUser())

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Run failed: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	tFile, err := os.CreateTemp(os.TempDir(), "gotee-example-*.txt")
	if err != nil {
		return fmt.Errorf("Cannot open tmp file: %v\n", tFile)
	}

	filePath := tFile.Name()

	fmt.Printf("Got tmp file: %s\n", filePath)

	longStr := "Very looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong str"
	execCmd := exec.Command("echo", longStr)

	bufFunc := make([]string, 0)

	i := 0
	fc := consumer.NewFuncNoErrConsumer(func(b []byte) {
		bufFunc = append(bufFunc, fmt.Sprintf("[%d] Consume: '%s'", i, string(b)))
		i++
	})

	wc := consumer.NewWriteCloserConsumer(tFile)

	results, err := cmd.RunCmd(
		context.TODO(),
		execCmd,
		cmd.RunCmdWithStdout(fc, wc),
		cmd.RunCmdWithReadBufSize(8),
	)

	resErr := results.Error()
	if err != nil || resErr != "" {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		if resErr != "" {
			errStr = fmt.Sprintf("%s\nResults:\n%s", errStr, resErr)
		}
		return fmt.Errorf("Run cmd failed: %s", errStr)
	}

	fmt.Printf("Function consumer result:\n%s\n", strings.Join(bufFunc, "\n\t"))

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("Cannot read file: %w", err)
	}

	fmt.Printf("Write consumer result:\n%s", string(content))

	os.Remove(filePath)

	return nil
}
