// DiningTable.java

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

/**
 * Orchestrates the dining table — creates forks, philosophers, and manages
 * their lifecycle via a fixed thread pool.
 *
 * Why ExecutorService instead of raw Thread[]?
 * - Centralized lifecycle management — single shutdown point
 * - Clean separation: DiningTable owns orchestration, Philosopher owns behavior
 * - awaitTermination gives a clean join-with-timeout pattern
 * - Easy to swap for a cached or scheduled pool without changing Philosopher
 *
 * Why not make DiningTable itself manage thread creation?
 * Single responsibility: DiningTable wires components together and manages
 * the simulation lifecycle. Thread scheduling is the ExecutorService's job.
 */
public class DiningTable {

    private static final Logger logger = Logger.getLogger(DiningTable.class.getName());

    private static final int PHILOSOPHER_COUNT = 5;
    private static final int DEFAULT_MEALS     = 3;
    private static final int SHUTDOWN_TIMEOUT_SECONDS = 30;

    private final Fork[] forks;
    private final List<Philosopher> philosophers;
    private final ExecutorService executorService;
    private final int mealsPerPhilosopher;

    public DiningTable(int mealsPerPhilosopher) {
        this.mealsPerPhilosopher = mealsPerPhilosopher;
        this.forks = createForks();
        this.philosophers = createPhilosophers();
        this.executorService = Executors.newFixedThreadPool(PHILOSOPHER_COUNT);
    }

    public DiningTable() {
        this(DEFAULT_MEALS);
    }

    // -------------------------------------------------------------------------
    // Lifecycle
    // -------------------------------------------------------------------------

    /**
     * Start the simulation. Submits all philosophers to the thread pool
     * and waits for all to finish.
     *
     * awaitTermination with a timeout prevents the simulation from hanging
     * indefinitely if something goes wrong — a production safety net.
     * If the timeout fires, we force-shutdown and log the anomaly.
     */
    public void start() {
        logger.info("=== Dining Table starting — " + PHILOSOPHER_COUNT
                + " philosophers, " + mealsPerPhilosopher + " meals each ===");

        for (Philosopher philosopher : philosophers) {
            executorService.submit(philosopher);
        }

        executorService.shutdown();

        try {
            boolean completed = executorService.awaitTermination(
                    SHUTDOWN_TIMEOUT_SECONDS, TimeUnit.SECONDS);

            if (completed) {
                logger.info("=== All philosophers finished ===");
                printSummary();
            } else {
                // This should never happen with correct deadlock prevention.
                // If it does, it signals a bug — log loudly.
                logger.severe("=== Simulation timed out — possible deadlock or starvation ===");
                executorService.shutdownNow();
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            logger.warning("Main thread interrupted — forcing shutdown");
            executorService.shutdownNow();
        }
    }

    // -------------------------------------------------------------------------
    // Wiring
    // -------------------------------------------------------------------------

    /**
     * Creates 5 forks numbered 0–4.
     * Index in the array IS the fork's ID — no separate mapping needed.
     */
    private Fork[] createForks() {
        Fork[] f = new Fork[PHILOSOPHER_COUNT];
        for (int i = 0; i < PHILOSOPHER_COUNT; i++) {
            f[i] = new Fork(i);
        }
        return f;
    }

    /**
     * Creates philosophers and assigns their adjacent forks.
     *
     * Fork assignment:
     *   Philosopher i → left fork = forks[i], right fork = forks[(i+1) % N]
     *
     * The Philosopher constructor handles lock ordering internally.
     * DiningTable only needs to know the physical adjacency — it does not
     * need to know which fork gets acquired first. That responsibility
     * belongs to Philosopher, not to the table.
     */
    private List<Philosopher> createPhilosophers() {
        List<Philosopher> list = new ArrayList<>(PHILOSOPHER_COUNT);
        for (int i = 0; i < PHILOSOPHER_COUNT; i++) {
            Fork leftFork  = forks[i];
            Fork rightFork = forks[(i + 1) % PHILOSOPHER_COUNT];
            list.add(new Philosopher(i, leftFork, rightFork, mealsPerPhilosopher));
        }
        return list;
    }

    // -------------------------------------------------------------------------
    // Observability
    // -------------------------------------------------------------------------

    private void printSummary() {
        System.out.println("\n=== Simulation Summary ===");
        int total = 0;
        for (Philosopher p : philosophers) {
            System.out.printf("  P%d — meals eaten: %d, final state: %s%n",
                    p.getId(), p.getMealsEaten(), p.getState());
            total += p.getMealsEaten();
        }
        System.out.printf("  Total meals: %d (expected: %d)%n",
                total, PHILOSOPHER_COUNT * mealsPerPhilosopher);
        System.out.println("==========================");
    }
}