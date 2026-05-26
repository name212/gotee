// Copyright 2026
// license that can be found in the LICENSE file.

package gotee_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"

	tee "github.com/name212/gotee"
	"github.com/stretchr/testify/require"
)

const (
	execTestDefaultBufKey    = "buf"
	execTestDefaultLineKey   = "line"
	execTestDefaultWriterKey = "writer"
)

func TestExec(t *testing.T) {
	suit := newTestExecSuit(t)

	suit.enableDebug(true)
	suit.runStdoutOnlyTests()
	suit.runStdoutAndErrTests()
	suit.runStderrOnlyTests()

	var (
		errStdoutWriter       = fmt.Errorf("errStdoutWriter")
		errStdoutWriterSecond = fmt.Errorf("secondErrStdoutWriter")
		errStderrWriter       = fmt.Errorf("errStderrWriter")
	)

	stdOutOnlyTests := []testExec{
		{
			name: "one buffer consumer",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				return returnDefaultBufConsumer(tst, "stdout_one_buf")
			},
			script: scriptOnlyStdout,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				assertDefaultBuffer(t, tst, `First string
Second string
Third string
`)
			},
		},

		{
			name: "multiple consumers",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultBufConsumer(tst, "stdout_multiple_buf")
				consumers = append(consumers, returnDefaultLineConsumer(tst, "stout_multiple_line")...)
				return append(consumers, returnDefaultWriterConsumer(tst, "stdout_multiple_writer")...)
			},
			script: scriptOnlyStdout,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				bufExpected := `First string
Second string
Third string
`
				assertDefaultBuffer(t, tst, bufExpected)
				assertDefaultLinesHandler(t, tst, []string{
					"First string",
					"Second string",
					"Third string",
				}...)
				assertDefaultWriterConsumer(t, tst, bufExpected)
			},
		},

		{
			name: "multiple consumers stdout with stderr",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultBufConsumer(tst, "stdout_both_multiple_buf")
				consumers = append(consumers, returnDefaultLineConsumer(tst, "stdout_both_multiple_line")...)
				return append(consumers, returnDefaultWriterConsumer(tst, "stdout_both_multiple_writer")...)
			},
			script: scriptStdoutAndStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				bufExpected := `First string
Second string
Third string
`
				assertDefaultBuffer(t, tst, bufExpected)
				assertDefaultLinesHandler(t, tst, []string{
					"First string",
					"Second string",
					"Third string",
				}...)
				assertDefaultWriterConsumer(t, tst, bufExpected)
			},
		},

		{
			name: "multiple consumers only stderr",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultBufConsumer(tst, "stdout_both_multiple_buf")
				consumers = append(consumers, returnDefaultLineConsumer(tst, "stdout_both_multiple_line")...)
				return append(consumers, returnDefaultWriterConsumer(tst, "stdout_both_multiple_writer")...)
			},
			script: scriptOnlyStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)

				assertDefaultBuffer(t, tst)
				assertDefaultLinesHandler(t, tst)
				assertDefaultWriterConsumer(t, tst)
			},
		},

		{
			name: "multiple consumers with one error",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultErrWriterConsumer(tst, "stdout_err_writer", func(b []byte) ([]byte, error) {
					cut := []byte("Second")
					if bytes.Contains(b, cut) {
						t.Logf("Return error %v", errStdoutWriter)
						return cut, errStdoutWriter
					}
					return cut, nil
				})

				consumers = append(consumers, returnDefaultLineConsumer(tst, "stdout_err_writer_line_all")...)

				return consumers
			},
			script: scriptStdoutAndStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results, errStdoutWriter)
				assertExecError(t, err, false)

				assertDefaultWriterConsumer(t, tst, "First string\n")
				assertDefaultLinesHandler(t, tst, []string{
					"First string",
					"Second string",
					"Third string",
				}...)
			},
		},

		{
			name: "multiple consumers with error multiple error sleep",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultErrWriterConsumer(tst, "stdout_err_writer_first", func(b []byte) ([]byte, error) {
					cut := []byte("Second")
					if bytes.Contains(b, cut) {
						t.Logf("Return error for first %v", errStdoutWriter)
						return cut, errStdoutWriter
					}
					return cut, nil
				})

				second := newExecErrWriterConsumer(tst, "stdout_err_writer_second", "second", func(b []byte) ([]byte, error) {
					cut := []byte("string")
					if bytes.Contains(b, cut) {
						t.Logf("Return error for second %v", errStdoutWriterSecond)
						return cut, errStdoutWriterSecond
					}
					return cut, nil
				})

				consumers = append(consumers, second)

				consumers = append(consumers, returnDefaultLineConsumer(tst, "stdout_err_writer_line_all_mul")...)

				return consumers
			},
			script: scriptStdOutAndErrWithSleep(3),
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(
					t,
					results,
					errStdoutWriter,
					errStdoutWriterSecond,
				)
				assertExecError(t, err, false)

				assertDefaultWriterConsumer(t, tst, "First string\n")
				assertDefaultLinesHandler(t, tst, []string{
					"First string",
					"Second string",
					"Third string",
				}...)

				assertWriterConsumer(t, tst.consumersData["second"], "First ")
			},
		},

		{
			name: "exec error",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				return returnDefaultBufConsumer(tst, "stout_exit_err")
			},
			script: scriptStdOutAndErrWithErrExit,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, true)
				assertDefaultBuffer(t, tst, `First string
Second string
`)
			},
		},
	}

	stdoutAndStdErrTests := []testExec{
		{
			name: "one buffer consumer",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				return returnDefaultBufConsumer(tst, "out_err_one_buf_out")
			},
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumer := newBufConsumer(tst, "out_err_one_buf_err", "stderr")
				return []tee.Consumer{consumer}
			},
			script: scriptStdoutAndStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				assertDefaultBuffer(t, tst, `First string
Second string
Third string
`)
				assertBuffer(t, tst.consumersData["stderr"], `Error first
Error second
Error third
`)
			},
		},

		{
			name: "multiple consumers",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultBufConsumer(tst, "out_err_mul_buf")
				consumers = append(consumers, returnDefaultLineConsumer(tst, "out_err_mul_line")...)
				return append(consumers, returnDefaultWriterConsumer(tst, "out_err_mul_writer")...)
			},
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				bufConsumer := newBufConsumer(tst, "out_err_mul_buf_err", "stderr_buf")
				lineConsumer := newLineConsumer(tst, "out_err_mul_line_err", "stderr_line")
				writerConsumer := newWriterConsumer(tst, "out_err_mul_writer", "stderr_writer")
				return []tee.Consumer{bufConsumer, lineConsumer, writerConsumer}
			},
			script: scriptStdoutAndStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				bufOutExpected := `First string
Second string
Third string
`
				bufErrExpected := `Error first
Error second
Error third
`
				stdoutLinesExpected := []string{
					"First string",
					"Second string",
					"Third string",
				}
				stderrLinesExpected := []string{
					"Error first",
					"Error second",
					"Error third",
				}

				assertDefaultBuffer(t, tst, bufOutExpected)
				assertDefaultLinesHandler(t, tst, stdoutLinesExpected...)
				assertDefaultWriterConsumer(t, tst, bufOutExpected)

				assertBuffer(t, tst.consumersData["stderr_buf"], bufErrExpected)
				assertStringLineHandler(t, tst.consumersData["stderr_line"], stderrLinesExpected...)
				assertWriterConsumer(t, tst.consumersData["stderr_writer"], bufErrExpected)
			},
		},

		{
			name: "only out stdout",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				return returnDefaultBufConsumer(tst, "out_err_only_out")
			},
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumer := newBufConsumer(tst, "out_err_only_out_err", "stderr")
				return []tee.Consumer{consumer}
			},
			script: scriptOnlyStdout,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				assertDefaultBuffer(t, tst, `First string
Second string
Third string
`)
				assertBuffer(t, tst.consumersData["stderr"])
			},
		},

		{
			name: "only out stderr",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				return returnDefaultBufConsumer(tst, "out_err_only_err_out")
			},
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumer := newBufConsumer(tst, "out_err_only_err_err", "stderr")
				return []tee.Consumer{consumer}
			},
			script: scriptOnlyStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				assertDefaultBuffer(t, tst)
				assertBuffer(t, tst.consumersData["stderr"], `Error first
Error second
Error third
`)
			},
		},

		{
			name: "multiple consumers with one error",
			stdoutConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultErrWriterConsumer(tst, "out_err_one_err_err_writer", func(b []byte) ([]byte, error) {
					cut := []byte("Second")
					if bytes.Contains(b, cut) {
						t.Logf("Return error %v", errStdoutWriter)
						return cut, errStdoutWriter
					}
					return cut, nil
				})

				consumers = append(consumers, returnDefaultLineConsumer(tst, "out_err_one_err_writer_line_all")...)

				return consumers
			},

			stderrConsumers: func(tst *testExec) []tee.Consumer {
				errConsumer := newExecErrWriterConsumer(tst, "out_err_one_err_err_writer_err", "stderr_err", func(b []byte) ([]byte, error) {
					cut := []byte(" first")
					if bytes.Contains(b, cut) {
						t.Logf("Return error %v", errStderrWriter)
						return cut, errStderrWriter
					}
					return cut, nil
				})

				linesConsumer := newLineConsumer(tst, "out_err_one_err_writer_line_all_err", "stderr_line")

				return []tee.Consumer{errConsumer, linesConsumer}
			},
			script: scriptStdoutAndStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results, errStdoutWriter)
				assertExecError(t, err, false)

				assertDefaultWriterConsumer(t, tst, "First string\n")
				assertDefaultLinesHandler(t, tst, []string{
					"First string",
					"Second string",
					"Third string",
				}...)

				assertWriterConsumer(t, tst.consumersData["stderr_err"], "Error")
				assertStringLineHandler(t, tst.consumersData["stderr_line"], []string{
					"Error first",
					"Error second",
					"Error third",
				}...)
			},
		},
	}

	stderrOnlyTests := []testExec{
		{
			name: "one buffer consumer",
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				return returnDefaultBufConsumer(tst, "stderr_one_buf")
			},
			script: scriptOnlyStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				assertDefaultBuffer(t, tst, `Error first
Error second
Error third
`)
			},
		},

		{
			name: "multiple consumers",
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultBufConsumer(tst, "stderr_multiple_buf")
				consumers = append(consumers, returnDefaultLineConsumer(tst, "stderr_multiple_line")...)
				return append(consumers, returnDefaultWriterConsumer(tst, "stderr_multiple_writer")...)
			},
			script: scriptOnlyStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				bufExpected := `Error first
Error second
Error third
`
				assertDefaultBuffer(t, tst, bufExpected)
				assertDefaultLinesHandler(t, tst, []string{
					"Error first",
					"Error second",
					"Error third",
				}...)
				assertDefaultWriterConsumer(t, tst, bufExpected)
			},
		},

		{
			name: "multiple consumers stdout with stderr",
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultBufConsumer(tst, "stderr_both_multiple_buf")
				consumers = append(consumers, returnDefaultLineConsumer(tst, "stderr_both_multiple_line")...)
				return append(consumers, returnDefaultWriterConsumer(tst, "stderr_both_multiple_writer")...)
			},
			script: scriptStdoutAndStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)
				bufExpected := `Error first
Error second
Error third
`
				assertDefaultBuffer(t, tst, bufExpected)
				assertDefaultLinesHandler(t, tst, []string{
					"Error first",
					"Error second",
					"Error third",
				}...)
				assertDefaultWriterConsumer(t, tst, bufExpected)
			},
		},

		{
			name: "multiple consumers only stdout",
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultBufConsumer(tst, "stderr_both_multiple_buf")
				consumers = append(consumers, returnDefaultLineConsumer(tst, "stderr_both_multiple_line")...)
				return append(consumers, returnDefaultWriterConsumer(tst, "stderr_both_multiple_writer")...)
			},
			script: scriptOnlyStdout,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, false)

				assertDefaultBuffer(t, tst)
				assertDefaultLinesHandler(t, tst)
				assertDefaultWriterConsumer(t, tst)
			},
		},

		{
			name: "multiple consumers with one error",
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultErrWriterConsumer(tst, "stderr_err_writer", func(b []byte) ([]byte, error) {
					cut := []byte(" first")
					if bytes.Contains(b, cut) {
						t.Logf("Return error %v", errStdoutWriter)
						return cut, errStdoutWriter
					}
					return cut, nil
				})

				consumers = append(consumers, returnDefaultLineConsumer(tst, "stdout_err_writer_line_all")...)

				return consumers
			},
			script: scriptOnlyStderr,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results, errStdoutWriter)
				assertExecError(t, err, false)

				assertDefaultWriterConsumer(t, tst, "Error")
				assertDefaultLinesHandler(t, tst, []string{
					"Error first",
					"Error second",
					"Error third",
				}...)
			},
		},

		{
			name: "multiple consumers with error multiple error sleep",
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				consumers := returnDefaultErrWriterConsumer(tst, "stderr_err_writer_first", func(b []byte) ([]byte, error) {
					cut := []byte(" first")
					if bytes.Contains(b, cut) {
						t.Logf("Return error for first %v", errStdoutWriter)
						return cut, errStdoutWriter
					}
					return cut, nil
				})

				second := newExecErrWriterConsumer(tst, "stderr_err_writer_second", "second", func(b []byte) ([]byte, error) {
					cut := []byte("or seco")
					if bytes.Contains(b, cut) {
						t.Logf("Return error for second %v", errStdoutWriterSecond)
						return cut, errStdoutWriterSecond
					}
					return cut, nil
				})

				consumers = append(consumers, second)

				consumers = append(consumers, returnDefaultLineConsumer(tst, "stderr_err_writer_line_all_mul")...)

				return consumers
			},
			script: scriptStdOutAndErrWithSleep(3),
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(
					t,
					results,
					errStdoutWriter,
					errStdoutWriterSecond,
				)
				assertExecError(t, err, false)

				assertDefaultWriterConsumer(t, tst, "Error")
				assertDefaultLinesHandler(t, tst, []string{
					"Error first",
					"Error second",
					"Error third",
				}...)

				assertWriterConsumer(t, tst.consumersData["second"], "Error first\nErr")
			},
		},

		{
			name: "exec error",
			stderrConsumers: func(tst *testExec) []tee.Consumer {
				return returnDefaultBufConsumer(tst, "sterr_exit_err")
			},
			script: scriptStdOutAndErrWithErrExit,
			assert: func(t *testing.T, tst *testExec, results *tee.Results, err error) {
				assertExecResults(t, results)
				assertExecError(t, err, true)
				assertDefaultBuffer(t, tst, `Error first
Error second
`)
			},
		},
	}

	t.Run("stdout only", func(t *testing.T) {
		for indx, tt := range stdOutOnlyTests {
			if suit.checkStdoutOnlyTestSkip(t, indx, tt.name) {
				continue
			}

			tt.run(t)
		}
	})

	t.Run("stdout and stderr", func(t *testing.T) {
		for indx, tt := range stdoutAndStdErrTests {
			if suit.checkStdoutAndErrTestSkip(t, indx, tt.name) {
				continue
			}

			tt.run(t)
		}
	})

	t.Run("stderr only", func(t *testing.T) {
		for indx, tt := range stderrOnlyTests {
			if suit.checkStderrOnlyTestSkip(t, indx, tt.name) {
				continue
			}

			tt.run(t)
		}
	})

}

type testExecSuit struct {
	mu         sync.Mutex
	hasSkipped bool

	root *testing.T
}

func newTestExecSuit(root *testing.T) *testExecSuit {
	return &testExecSuit{
		root: root,
	}
}

func (s *testExecSuit) setHasSkipped() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hasSkipped {
		return
	}

	s.root.Cleanup(func() {
		msg := "FAILED"

		if !s.root.Failed() {
			msg = "!! OK But have skipped tests !!"
		}

		s.root.Error(msg)
	})
}

func (s *testExecSuit) enableDebug(enable bool) {
	if enable {
		enableDebugLogs(s.root)
	}
}

func (s *testExecSuit) fillRunTestsEnv(envName string, numbers ...int) {
	if len(numbers) == 0 {
		return
	}

	strs := make([]string, 0, len(numbers))
	for _, n := range numbers {
		strs = append(strs, fmt.Sprintf("%d", n))
	}

	s.root.Setenv(envName, strings.Join(strs, ","))
}

func (s *testExecSuit) parseAndCheckTestsForRun(t *testing.T, envName string, indx int, name string) bool {
	runOnlyTests := os.Getenv(envName)

	if runOnlyTests == "" {
		return false
	}

	if runOnlyTests == "-1" {
		return true
	}

	numbersStrs := strings.Split(runOnlyTests, ",")

	toRun := make(map[int]struct{})
	for _, s := range numbersStrs {
		s = strings.TrimSpace(s)
		runTst, err := strconv.Atoi(s)
		if err != nil {
			continue
		}

		toRun[runTst] = struct{}{}
	}

	if _, ok := toRun[indx]; !ok {
		t.Logf("!!!!!! Skip %s test %s because run only %v !!!!", envName, name, toRun)
		s.setHasSkipped()
		return true
	}

	return false
}

func (s *testExecSuit) runStdoutOnlyTests(numbers ...int) {
	s.fillRunTestsEnv("RUN_STD_ONLY_TEST", numbers...)
}

func (s *testExecSuit) checkStdoutOnlyTestSkip(t *testing.T, indx int, name string) bool {
	return s.parseAndCheckTestsForRun(t, "RUN_STD_ONLY_TEST", indx, name)
}

func (s *testExecSuit) runStdoutAndErrTests(numbers ...int) {
	s.fillRunTestsEnv("RUN_STD_OUT_AND_ERR_TEST", numbers...)
}

func (s *testExecSuit) checkStdoutAndErrTestSkip(t *testing.T, indx int, name string) bool {
	return s.parseAndCheckTestsForRun(t, "RUN_STD_OUT_AND_ERR_TEST", indx, name)
}

func (s *testExecSuit) runStderrOnlyTests(numbers ...int) {
	s.fillRunTestsEnv("RUN_STD_OUT_AND_ERR_TEST", numbers...)
}

func (s *testExecSuit) checkStderrOnlyTestSkip(t *testing.T, indx int, name string) bool {
	return s.parseAndCheckTestsForRun(t, "RUN_STD_ERR_ONLY_TEST", indx, name)
}

type testExec struct {
	name            string
	stdoutConsumers func(*testExec) []tee.Consumer
	stderrConsumers func(*testExec) []tee.Consumer
	bufSize         int
	consumersData   map[string]any
	assert          func(*testing.T, *testExec, *tee.Results, error)
	runInGorutine   func(*testing.T, *testExec)
	script          string
	scriptArgs      []string
}

func (tt *testExec) run(t *testing.T) {
	t.Run(tt.name, func(t *testing.T) {
		require.NotEmpty(t, tt.script, "script should passed")
		require.NotNil(t, tt.assert, "assert should passed")

		replaces := []string{
			" ",
			"/",
			"\\",
			":",
			`"`,
			`'`,
			`*`,
			`?`,
			`$`,
			`#`,
		}

		scriptName := tt.name
		for _, toReplace := range replaces {
			scriptName = strings.ReplaceAll(scriptName, toReplace, "_")
		}

		scriptPath := writeScript(t, scriptName, tt.script)

		tt.consumersData = make(map[string]any)

		opts := make([]tee.RunCmdOpt, 0, 3)

		if tt.stdoutConsumers != nil {
			opts = append(opts, tee.RunCmdWithStdout(tt.stdoutConsumers(tt)...))
		}

		if tt.stderrConsumers != nil {
			opts = append(opts, tee.RunCmdWithStderr(tt.stderrConsumers(tt)...))
		}

		if tt.bufSize > 0 {
			opts = append(opts, tee.RunCmdWithReadBufSize(tt.bufSize))
		}

		cmd := exec.Command(scriptPath, tt.scriptArgs...)
		if tt.runInGorutine != nil {
			go func() {
				tt.runInGorutine(t, tt)
			}()
		}

		results, err := tee.RunCmd(t.Context(), cmd, opts...)

		tt.assert(t, tt, results, err)

		assertNoTeeGorutines(t, map[string]string{
			"created by RunCmd": "created by github.com/name212/gotee.RunCmd",
		})

	})
}

func newBufConsumer(tst *testExec, name, bufKey string) tee.Consumer {
	buf := &bytes.Buffer{}
	tst.consumersData[bufKey] = buf
	return tee.NewBufferConsumer(buf, name)
}

func returnDefaultBufConsumer(tst *testExec, name string) []tee.Consumer {
	return []tee.Consumer{newBufConsumer(tst, name, execTestDefaultBufKey)}
}

func newLineConsumer(tst *testExec, name, handlerKey string) tee.Consumer {
	lineHandler := tee.NewStringsSliceLineHandler()
	tst.consumersData[handlerKey] = lineHandler
	return tee.NewLineConsumer(lineHandler, name)
}

func returnDefaultLineConsumer(tst *testExec, name string) []tee.Consumer {
	return []tee.Consumer{newLineConsumer(tst, name, execTestDefaultLineKey)}
}

func newWriterConsumer(tst *testExec, name, writerKey string) tee.Consumer {
	consumer := newTestWriteCloserConsumer(name)
	tst.consumersData[writerKey] = consumer
	return consumer
}

func returnDefaultWriterConsumer(tst *testExec, name string) []tee.Consumer {
	return []tee.Consumer{newWriterConsumer(tst, name, execTestDefaultWriterKey)}
}

func newExecErrWriterConsumer(tst *testExec, name, key string, checker func([]byte) ([]byte, error)) tee.Consumer {
	consumer := newTestWriteCloserConsumer(name)
	consumer.setWriteErrChecker(checker)
	tst.consumersData[key] = consumer

	return consumer
}

func returnDefaultErrWriterConsumer(tst *testExec, name string, checker func([]byte) ([]byte, error)) []tee.Consumer {
	return []tee.Consumer{newExecErrWriterConsumer(tst, name, execTestDefaultWriterKey, checker)}
}

func assertDefaultBuffer(t *testing.T, tst *testExec, expected ...string) {
	buf := tst.consumersData[execTestDefaultBufKey]
	assertBuffer(t, buf, expected...)
}

func assertDefaultLinesHandler(t *testing.T, tst *testExec, expectedLines ...string) {
	handler := tst.consumersData[execTestDefaultLineKey]
	assertStringLineHandler(t, handler, expectedLines...)
}

func assertDefaultWriterConsumer(t *testing.T, tst *testExec, expectedLines ...string) {
	handler := tst.consumersData[execTestDefaultWriterKey]
	assertWriterConsumer(t, handler, expectedLines...)
}

func assertWriterConsumer(t *testing.T, rawConsumer any, expected ...string) {
	consumer, ok := rawConsumer.(*testWriteCloserConsumer)
	require.True(t, ok, "should be testWriteCloserConsumer")

	require.True(t, consumer.IsClosed(), "consumer should be closed")

	testBuffer := &bytes.Buffer{}
	testBuffer.WriteString(consumer.content())

	assertBuffer(t, testBuffer, expected...)
}

func assertStringLineHandler(t *testing.T, rawLine any, expectedLines ...string) {
	handler, ok := rawLine.(*tee.StringsSliceLineHandler)
	require.True(t, ok, "should be StringsSliceLineHandler")

	consumedLines := handler.Lines()
	require.Len(t, consumedLines, len(expectedLines), "lines handler should contains all lines")

	for indx, expected := range expectedLines {
		require.Equal(t, expected, consumedLines[indx], "incorrect consumed line %d", indx)
	}
}

func assertBuffer(t *testing.T, rawBuf any, expected ...string) {
	buf, ok := rawBuf.(*bytes.Buffer)
	require.True(t, ok, "should be buffer")

	content := buf.String()

	switch len(expected) {
	case 0:
		require.Empty(t, content, "buffer should not contains any")
		return
	case 1:
		require.Equal(t, expected[0], content, "buffer should equal")
	default:
		for _, e := range expected {
			require.Contains(t, content, e, "should contains buffer")
		}
	}
}

func assertExecResults(t *testing.T, r *tee.Results, contains ...error) {
	if len(contains) > 0 {
		require.NotNil(t, r, "results should not nil")
		for _, c := range contains {
			require.ErrorIs(t, r.GetError(), c, "results should contain err")
		}

		return
	}

	if r != nil {
		require.Nil(t, r, "results should be nil got %s", r.Error())
	}

	require.Nil(t, r, "results should be nil")
}

func assertExecError(t *testing.T, err error, shouldBe bool) {
	if shouldBe {
		require.Error(t, err, "exec should have error")
		return
	}

	require.NoError(t, err, "exec should not have error")
}

var (
	scriptOnlyStdout = `#!/usr/bin/env bash
echo "First string"
echo "Second string"
echo "Third string"
`

	scriptOnlyStderr = `#!/usr/bin/env bash
echo "Error first" >&2
echo "Error second" >&2
echo "Error third" >&2
`

	scriptStdoutAndStderr = `#!/usr/bin/env bash
echo "First string"
echo "Error first" >&2
echo "Second string"
echo "Error second" >&2
echo "Third string"
echo "Error third" >&2
`
	scriptStdOutAndErrWithErrExit = `#!/usr/bin/env bash
echo "First string"
echo "Error first" >&2
echo "Second string"
echo "Error second" >&2
exit 1
`
)

func scriptStdOutAndErrWithSleep(seconds int) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
echo "First string"
echo "Error first" >&2
echo "Second string"
sleep %d
echo "Error second" >&2
echo "Third string"
echo "Error third" >&2
`, seconds)
}
