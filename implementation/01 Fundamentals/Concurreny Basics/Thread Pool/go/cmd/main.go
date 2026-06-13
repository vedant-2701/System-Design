package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"threadpool"
	"time"
)

func main() {
	fmt.Println("=== Interactive Go Thread Pool Demo ===")
	fmt.Println("Initializing pool with Size: 2, Queue Capacity: 3, Rejection Policy: AbortPolicy")
	
	// Create the thread pool
	pool := threadpool.New(threadpool.Config{
		PoolSize:        2,
		QueueCapacity:   3,
		RejectionPolicy: threadpool.AbortPolicy,
	})

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\nCommands:")
	fmt.Println("  submit <id> <delay_ms> - Submit a task that sleeps for delay_ms")
	fmt.Println("  status                - View pool stats (active workers, queue size)")
	fmt.Println("  shutdown              - Gracefully shut down and wait")
	fmt.Println("  shutdownnow           - Immediately shut down")
	fmt.Println("  help                  - Print this menu")
	fmt.Println("  exit                  - Exit the program")

	for {
		fmt.Print("\n> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		parts := strings.Split(input, " ")
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "submit":
			if len(parts) < 3 {
				fmt.Println("Error: submit requires <id> and <delay_ms>")
				continue
			}
			id := parts[1]
			delayMs, err := strconv.Atoi(parts[2])
			if err != nil {
				fmt.Println("Error: invalid delay_ms")
				continue
			}

			task := func() {
				fmt.Printf("[Task %s] Starting (will run for %dms)\n", id, delayMs)
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
				fmt.Printf("[Task %s] Completed\n", id)
			}

			err = pool.Submit(task)
			if err != nil {
				fmt.Printf("Submit Rejected: %v\n", err)
			} else {
				fmt.Printf("Submit Accepted: Task %s\n", id)
			}

		case "status":
			fmt.Printf("Pool Status:\n")
			fmt.Printf("  Active Workers: %d\n", pool.ActiveWorkerCount())
			fmt.Printf("  Queue Size:     %d\n", pool.QueueSize())
			fmt.Printf("  Is Shutdown:    %v\n", pool.IsShutdown())

		case "shutdown":
			fmt.Println("Initiating graceful shutdown...")
			pool.Shutdown()
			fmt.Println("Waiting for workers to finish...")
			pool.Wait()
			fmt.Println("Pool fully terminated.")
			return

		case "shutdownnow":
			fmt.Println("Initiating immediate shutdown...")
			pool.ShutdownNow()
			fmt.Println("Waiting for workers to exit...")
			pool.Wait()
			fmt.Println("Pool terminated.")
			return

		case "help":
			fmt.Println("Commands:")
			fmt.Println("  submit <id> <delay_ms> - Submit a task that sleeps for delay_ms")
			fmt.Println("  status                - View pool stats (active workers, queue size)")
			fmt.Println("  shutdown              - Gracefully shut down and wait")
			fmt.Println("  shutdownnow           - Immediately shut down")
			fmt.Println("  help                  - Print this menu")
			fmt.Println("  exit                  - Exit the program")

		case "exit":
			if !pool.IsShutdown() {
				pool.ShutdownNow()
				pool.Wait()
			}
			return

		default:
			fmt.Printf("Unknown command: %s. Type 'help' for commands.\n", cmd)
		}
	}
}
