// PhilosopherState.java

/**
 * Explicit state enum for a philosopher's lifecycle.
 *
 * Using an enum instead of string literals:
 * - prevents typos in state comparisons
 * - makes state transitions visible and auditable
 * - enables exhaustive switch statements
 * - simplifies logging and debugging
 */
public enum PhilosopherState {
    THINKING,
    HUNGRY,       // wants to eat, waiting for forks
    EATING,
    DONE          // completed all meals, exiting
}