// Philosopher.java

import java.util.concurrent.ThreadLocalRandom;
import java.util.logging.Logger;

/**
 * Represents a philosopher who thinks and eats in a loop.
 *
 * Deadlock Prevention — Lock Ordering:
 * Each philosopher always acquires the lower-numbered fork first.
 * This breaks circular wait: P4 acquires fork 0 before fork 4,
 * competing with P0 for fork 0 instead of creating a cycle.
 *
 * Why Runnable and not Thread:
 * Separating the task (Philosopher) from the execution mechanism (Thread)
 * follows the single responsibility principle. The philosopher doesn't
 * need to know how it's scheduled — a thread pool could run it equally well.
 */
public class Philosopher implements Runnable {

    private static final Logger logger = Logger.getLogger(Philosopher.class.getName());

    private static final int MIN_THINK_MS = 100;
    private static final int MAX_THINK_MS = 500;
    private static final int MIN_EAT_MS   = 100;
    private static final int MAX_EAT_MS   = 300;

    private final int id;
    private final Fork firstFork;   // lower-numbered fork — always acquired first
    private final Fork secondFork;  // higher-numbered fork — acquired second
    private final int totalMeals;

    private volatile PhilosopherState state;
    private int mealsEaten;

    /**
     * @param id          philosopher index (0–4)
     * @param leftFork    the fork on the philosopher's left
     * @param rightFork   the fork on the philosopher's right
     * @param totalMeals  how many meals before this philosopher stops
     */
    public Philosopher(int id, Fork leftFork, Fork rightFork, int totalMeals) {
        this.id = id;
        this.totalMeals = totalMeals;
        this.state = PhilosopherState.THINKING;
        this.mealsEaten = 0;

        // Lock ordering: always assign lower-ID fork as firstFork.
        // This is the entire deadlock prevention mechanism — one assignment decision.
        if (leftFork.getId() < rightFork.getId()) {
            this.firstFork  = leftFork;
            this.secondFork = rightFork;
        } else {
            this.firstFork  = rightFork;
            this.secondFork = leftFork;
        }
    }

    @Override
    public void run() {
        logger.info(String.format("P%d starting — will eat %d meals", id, totalMeals));

        try {
            while (mealsEaten < totalMeals) {
                think();
                acquireForks();
                eat();
                releaseForks();
            }
        } catch (InterruptedException e) {
            // Restore interrupt flag and exit cleanly.
            // Swallowing InterruptedException without restoring the flag
            // prevents the thread pool or caller from knowing this thread
            // was interrupted — a common production bug.
            Thread.currentThread().interrupt();
            logger.warning(String.format("P%d interrupted after %d meals", id, mealsEaten));
        }

        transitionTo(PhilosopherState.DONE);
        logger.info(String.format("P%d finished — ate %d meals", id, mealsEaten));
    }

    // -------------------------------------------------------------------------
    // Core lifecycle steps
    // -------------------------------------------------------------------------

    private void think() throws InterruptedException {
        transitionTo(PhilosopherState.THINKING);
        int duration = randomBetween(MIN_THINK_MS, MAX_THINK_MS);
        logger.fine(String.format("P%d thinking for %dms", id, duration));
        Thread.sleep(duration);
    }

    /**
     * Acquire both forks in strict lower-ID-first order.
     *
     * firstFork.pickUp() blocks if a neighbor holds it — the OS parks
     * this thread and wakes it when the fork is released. No CPU burned
     * while waiting. No retry loop needed.
     *
     * There is a window between acquiring firstFork and acquiring secondFork
     * where this philosopher holds one fork. This is safe — lock ordering
     * prevents any circular wait from forming across all philosophers.
     */
    private void acquireForks() {
        transitionTo(PhilosopherState.HUNGRY);
        logger.fine(String.format("P%d hungry — waiting for %s then %s",
                id, firstFork, secondFork));

        firstFork.pickUp();
        logger.fine(String.format("P%d acquired %s", id, firstFork));

        secondFork.pickUp();
        logger.fine(String.format("P%d acquired %s — ready to eat", id, secondFork));
    }

    private void eat() throws InterruptedException {
        transitionTo(PhilosopherState.EATING);
        mealsEaten++;
        int duration = randomBetween(MIN_EAT_MS, MAX_EAT_MS);
        logger.info(String.format("P%d eating meal %d/%d for %dms",
                id, mealsEaten, totalMeals, duration));
        Thread.sleep(duration);
    }

    /**
     * Release in reverse acquisition order.
     *
     * Not strictly required for correctness with ReentrantLock,
     * but releasing in reverse order is a good discipline — it mirrors
     * RAII patterns and makes the pairing explicit in code.
     */
    private void releaseForks() {
        secondFork.putDown();
        firstFork.putDown();
        logger.fine(String.format("P%d released both forks", id));
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    private void transitionTo(PhilosopherState newState) {
        this.state = newState;
    }

    private int randomBetween(int min, int max) {
        return ThreadLocalRandom.current().nextInt(min, max + 1);
    }

    public int getId()            { return id; }
    public int getMealsEaten()    { return mealsEaten; }
    public PhilosopherState getState() { return state; }
}