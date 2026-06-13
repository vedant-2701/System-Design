// main demonstrates the process pool with three scenarios:
//   1. Normal task execution across multiple workers
//   2. Handling of failed commands (non-zero exit codes)
//   3. Worker crash recovery (worker killed mid-operation)
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"processpool/pool"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	workerBinary, err := buildWorkerBinary()
	if err != nil {
		log.Fatalf("build worker binary: %v", err)
	}
	defer os.Remove(workerBinary)

	fmt.Println("\n=== Scenario 1: Normal Task Execution ===")
	runNormalTasks(workerBinary)

	fmt.Println("\n=== Scenario 2: Failed Commands ===")
	runFailedCommands(workerBinary)

	fmt.Println("\n=== Scenario 3: Concurrent Load ===")
	runConcurrentLoad(workerBinary)
}

// runNormalTasks submits simple commands and collects results.
func runNormalTasks(workerBinary string) {
	p, err := pool.New(workerBinary, 3, 10)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer p.Stop()

	commands := []string{
		"echo hello from worker",
		"hostname",
		"date",
		"echo $(( 6 * 7 ))",
		"uname -s",
	}
	if runtime.GOOS == "windows" {
		commands = []string{
			"echo hello from worker",
			"hostname",
			"date /t",
			"echo 42",
			"ver",
		}
	}

	// Submit all tasks
	var taskIDs []string
	for _, cmd := range commands {
		id, err := p.Submit(cmd)
		if err != nil {
			log.Printf("submit error: %v", err)
			continue
		}
		taskIDs = append(taskIDs, id)
		log.Printf("submitted task id=%s cmd=%q", id, cmd)
	}

	// Collect results — must mark worker idle after each result
	// so pool knows the worker is available for next task.
	collected := 0
	timeout := time.After(10 * time.Second)

	for collected < len(taskIDs) {
		select {
		case result, ok := <-p.Results():
			if !ok {
				return
			}
			p.MarkWorkerIdleByPID(result.WorkerPID)
			if result.Err != nil {
				fmt.Printf("  task=%s ERROR: %v | output: %s\n",
					result.TaskID, result.Err, result.Output)
			} else {
				fmt.Printf("  task=%s OK: %s\n", result.TaskID, result.Output)
			}
			collected++

		case <-timeout:
			log.Printf("timeout waiting for results, got %d/%d",
				collected, len(taskIDs))
			return
		}
	}
}

// runFailedCommands demonstrates that worker survives command failures.
// A non-zero exit command does NOT kill the worker — the worker
// captures the error and reports it, then stays alive for next task.
// This is the isolation benefit: command failure ≠ worker failure.
func runFailedCommands(workerBinary string) {
	p, err := pool.New(workerBinary, 2, 10)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer p.Stop()

	commands := []string{
		"ls /nonexistent/path",        // exits with error
		"exit 1",                      // explicit failure
		"echo recovered && echo fine", // pool still works after failures
	}

	for _, cmd := range commands {
		id, _ := p.Submit(cmd)
		log.Printf("submitted task id=%s cmd=%q", id, cmd)
	}

	collected := 0
	timeout := time.After(10 * time.Second)

	for collected < len(commands) {
		select {
		case result, ok := <-p.Results():
			if !ok {
				return
			}
			p.MarkWorkerIdleByPID(result.WorkerPID)
			status := "OK"
			if result.Err != nil {
				status = fmt.Sprintf("ERR(%v)", result.Err)
			}
			fmt.Printf("  task=%s status=%s output=%q\n",
				result.TaskID, status, result.Output)
			collected++

		case <-timeout:
			log.Printf("timeout: got %d/%d results", collected, len(commands))
			return
		}
	}

	fmt.Printf("  pool still healthy: %d workers alive\n", p.WorkerCount())
}

// runConcurrentLoad submits many tasks simultaneously and measures throughput.
// This shows the pool handling real concurrency — multiple goroutines
// submitting tasks, pool dispatching to workers, results arriving out of order.
func runConcurrentLoad(workerBinary string) {
	workerCount := runtime.NumCPU()
	if workerCount > 4 {
		workerCount = 4
	}

	p, err := pool.New(workerBinary, workerCount, 50)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer p.Stop()

	taskCount := 20
	start := time.Now()

	// Submit concurrently from multiple goroutines — simulates real workload
	var submitWg sync.WaitGroup
	for i := range taskCount {
		submitWg.Add(1)
		go func(n int) {
			defer submitWg.Done()
			cmd := fmt.Sprintf("echo task-%d && sleep 0.05", n)
			if runtime.GOOS == "windows" {
				cmd = fmt.Sprintf("echo task-%d && ping 192.0.2.1 -n 1 -w 50 >nul", n)
			}
			id, err := p.Submit(cmd)
			if err != nil {
				log.Printf("submit %d error: %v", n, err)
				return
			}
			_ = id
		}(i)
	}
	submitWg.Wait()

	// Collect all results
	collected := 0
	timeout := time.After(30 * time.Second)

	for collected < taskCount {
		select {
		case result, ok := <-p.Results():
			if !ok {
				goto done
			}
			p.MarkWorkerIdleByPID(result.WorkerPID)
			collected++
		case <-timeout:
			log.Printf("timeout: collected %d/%d", collected, taskCount)
			goto done
		}
	}

done:
	elapsed := time.Since(start)
	fmt.Printf("  %d tasks, %d workers, elapsed=%v\n",
		taskCount, workerCount, elapsed.Round(time.Millisecond))

	// Theoretical minimum with perfect parallelism:
	// taskCount * 50ms / workerCount
	theoretical := time.Duration(taskCount) * 50 * time.Millisecond / time.Duration(workerCount)
	fmt.Printf("  theoretical minimum=%v (%.1fx speedup achieved)\n",
		theoretical.Round(time.Millisecond),
		float64(theoretical)/float64(elapsed))
}

// buildWorkerBinary compiles the worker binary to a temp file.
// This mirrors what production systems do — build a self-contained
// binary rather than running interpreted scripts.
func buildWorkerBinary() (string, error) {
	// Find the worker source relative to this file
	_, currentFile, _, _ := runtime.Caller(0)
	workerSrc := filepath.Join(filepath.Dir(currentFile), "worker")

	outPath := filepath.Join(os.TempDir(), "pool_worker")
	if runtime.GOOS == "windows" {
		outPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", outPath, workerSrc)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build worker: %w", err)
	}

	return outPath, nil
}