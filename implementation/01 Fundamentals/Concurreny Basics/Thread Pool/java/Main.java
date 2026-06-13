import java.util.Scanner;
import java.util.concurrent.TimeUnit;

public class Main {
    public static void main(String[] args) {
        System.out.println("=== Interactive Java Thread Pool Demo ===");
        System.out.println("Initializing pool with Size: 2, Queue Capacity: 3, Rejection Policy: AbortPolicy");

        // Create the thread pool
        FixedThreadPool pool = FixedThreadPool.builder()
                .poolSize(2)
                .queueCapacity(3)
                .rejectionPolicy(new RejectionPolicy.AbortPolicy())
                .build();

        Scanner scanner = new Scanner(System.in);
        System.out.println("\nCommands:");
        System.out.println("  submit <id> <delay_ms> - Submit a task that sleeps for delay_ms");
        System.out.println("  status                - View pool stats (active workers, queue size)");
        System.out.println("  shutdown              - Gracefully shut down and wait");
        System.out.println("  shutdownnow           - Immediately shut down");
        System.out.println("  help                  - Print this menu");
        System.out.println("  exit                  - Exit the program");

        while (true) {
            System.out.print("\n> ");
            if (!scanner.hasNextLine()) {
                break;
            }
            String input = scanner.nextLine().trim();
            if (input.isEmpty()) {
                continue;
            }

            String[] parts = input.split("\\s+");
            String cmd = parts[0].toLowerCase();

            switch (cmd) {
                case "submit":
                    if (parts.length < 3) {
                        System.out.println("Error: submit requires <id> and <delay_ms>");
                        continue;
                    }
                    String id = parts[1];
                    int delayMs;
                    try {
                        delayMs = Integer.parseInt(parts[2]);
                    } catch (NumberFormatException e) {
                        System.out.println("Error: invalid delay_ms");
                        continue;
                    }

                    Runnable task = () -> {
                        System.out.printf("[Task %s] Starting (will run for %dms)\n", id, delayMs);
                        try {
                            Thread.sleep(delayMs);
                        } catch (InterruptedException e) {
                            System.out.printf("[Task %s] Interrupted!\n", id);
                            Thread.currentThread().interrupt();
                            return;
                        }
                        System.out.printf("[Task %s] Completed\n", id);
                    };

                    try {
                        pool.submit(task);
                        System.out.printf("Submit Accepted: Task %s\n", id);
                    } catch (RejectedTaskException e) {
                        System.out.printf("Submit Rejected: %s\n", e.getMessage());
                    } catch (NullPointerException e) {
                        System.out.println("Error: task is null");
                    }
                    break;

                case "status":
                    System.out.println("Pool Status:");
                    System.out.printf("  Active Workers: %d\n", pool.getActiveWorkerCount());
                    System.out.printf("  Queue Size:     %d\n", pool.getQueueSize());
                    System.out.printf("  Is Shutdown:    %b\n", pool.isShutdown());
                    System.out.printf("  Is Terminated:  %b\n", pool.isTerminated());
                    break;

                case "shutdown":
                    System.out.println("Initiating graceful shutdown...");
                    pool.shutdown();
                    System.out.println("Waiting for workers to finish (timeout 30s)...");
                    try {
                        boolean clean = pool.awaitTermination(30, TimeUnit.SECONDS);
                        System.out.printf("Pool terminated cleanly: %b\n", clean);
                    } catch (InterruptedException e) {
                        System.out.println("Interrupted waiting for termination.");
                        Thread.currentThread().interrupt();
                    }
                    return;

                case "shutdownnow":
                    System.out.println("Initiating immediate shutdown...");
                    pool.shutdownNow();
                    System.out.println("Waiting for workers to exit (timeout 5s)...");
                    try {
                        boolean clean = pool.awaitTermination(5, TimeUnit.SECONDS);
                        System.out.printf("Pool terminated cleanly: %b\n", clean);
                    } catch (InterruptedException e) {
                        System.out.println("Interrupted waiting for termination.");
                        Thread.currentThread().interrupt();
                    }
                    return;

                case "help":
                    System.out.println("Commands:");
                    System.out.println("  submit <id> <delay_ms> - Submit a task that sleeps for delay_ms");
                    System.out.println("  status                - View pool stats (active workers, queue size)");
                    System.out.println("  shutdown              - Gracefully shut down and wait");
                    System.out.println("  shutdownnow           - Immediately shut down");
                    System.out.println("  help                  - Print this menu");
                    System.out.println("  exit                  - Exit the program");
                    break;

                case "exit":
                    if (!pool.isShutdown()) {
                        pool.shutdownNow();
                        try {
                            pool.awaitTermination(5, TimeUnit.SECONDS);
                        } catch (InterruptedException e) {
                            Thread.currentThread().interrupt();
                        }
                    }
                    return;

                default:
                    System.out.printf("Unknown command: %s. Type 'help' for commands.\n", cmd);
                    break;
            }
        }
    }
}
