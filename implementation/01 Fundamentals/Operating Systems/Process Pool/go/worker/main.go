// worker is a standalone process managed by the pool.
// It communicates exclusively via stdin/stdout — no shared memory,
// no network. This isolation means a worker crash cannot corrupt
// the pool manager or other workers.
//
// Protocol (line-delimited, intentionally simple):
//   stdin  ← "TASK:<id>:<command>\n"
//   stdout → "RESULT:<id>:OK:<output>\n"   on success
//   stdout → "RESULT:<id>:ERR:<message>\n" on failure
//   stdin  ← "SHUTDOWN\n"                  signals clean exit
//
// Why line-delimited? Pipes are byte streams — no message boundaries.
// Newlines give us a simple framing protocol without a binary encoding
// library. Production systems use length-prefixed binary or protobuf,
// but that adds complexity that obscures the OS concepts here.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func main() {
	// bufio.NewReader batches reads — avoids one read() syscall per byte.
	// Without this, reading line-by-line from a pipe would be extremely
	// inefficient — exactly the problem strace showed us.
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF means the pool manager closed the write end of our
			// task pipe — either it crashed or cleanly shut down.
			// Either way, worker should exit. This is the EOF-as-signal
			// pattern we saw with pipes in strace.
			os.Exit(0)
		}

		line = strings.TrimSpace(line)

		if line == "SHUTDOWN" {
			os.Exit(0)
		}

		if !strings.HasPrefix(line, "TASK:") {
			// Malformed message — log to stderr (fd 2, not captured by pool)
			// and continue. Never crash on bad input.
			fmt.Fprintf(os.Stderr, "worker: malformed message: %q\n", line)
			continue
		}

		taskID, command, err := parseTask(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "worker: parse error: %v\n", err)
			continue
		}

		output, execErr := executeCommand(command)
		writeResult(os.Stdout, taskID, output, execErr)
	}
}

// parseTask extracts taskID and command from "TASK:<id>:<command>".
// Command may contain colons (e.g. "curl http://host:8080") so we
// split on the first two colons only.
func parseTask(line string) (taskID string, command string, err error) {
	// Strip "TASK:" prefix
	rest := strings.TrimPrefix(line, "TASK:")

	// Split into id and command on first colon only
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected TASK:<id>:<command>, got: %q", line)
	}

	taskID = strings.TrimSpace(parts[0])
	command = strings.TrimSpace(parts[1])

	if taskID == "" || command == "" {
		return "", "", fmt.Errorf("empty taskID or command in: %q", line)
	}

	return taskID, command, nil
}

// executeCommand runs a shell command and captures combined output.
// We use "sh -c" so commands can use pipes, redirects, etc.
// CombinedOutput captures both stdout and stderr from the command —
// gives the pool manager full visibility into what happened.
func executeCommand(command string) (output string, err error) {
	// #nosec G204 — shell injection is intentional here; this is a
	// command execution pool. In production, you would validate or
	// allowlist commands before this point.
	shell := "sh"
	flag := "-c"
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	}
	cmd := exec.Command(shell, flag, command)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// writeResult writes a result line to stdout.
// Flush is guaranteed because fmt.Fprintf writes directly to os.Stdout
// (unbuffered). If we used bufio here, we'd need explicit flushes —
// a common bug in pipe-based IPC where the reader blocks forever
// waiting for data stuck in a write buffer.
func writeResult(out *os.File, taskID, output string, execErr error) {
	// Sanitize output — newlines would break our line protocol
	sanitized := strings.ReplaceAll(output, "\n", "\\n")

	if execErr != nil {
		errMsg := strings.ReplaceAll(execErr.Error(), "\n", "\\n")
		fmt.Fprintf(out, "RESULT:%s:ERR:%s|%s\n", taskID, errMsg, sanitized)
	} else {
		fmt.Fprintf(out, "RESULT:%s:OK:%s\n", taskID, sanitized)
	}
}