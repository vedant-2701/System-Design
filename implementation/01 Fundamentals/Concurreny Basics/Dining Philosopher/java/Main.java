// Main.java

import java.util.logging.ConsoleHandler;
import java.util.logging.Level;
import java.util.logging.Logger;
import java.util.logging.SimpleFormatter;

/**
 * Entrypoint. Configures logging and starts the simulation.
 *
 * Logging levels used throughout:
 *   INFO  — state transitions visible to the user (eating, done)
 *   FINE  — detailed fork acquisition steps (enable for debugging)
 *   SEVERE — anomalies that should never happen in correct execution
 *
 * Set level to FINE to trace every fork pickup/putdown.
 * Set level to INFO for clean readable output.
 */
public class Main {

    public static void main(String[] args) {
        configureLogging(Level.INFO);

        int meals = parseMeals(args);
        DiningTable table = new DiningTable(meals);
        table.start();
    }

    private static void configureLogging(Level level) {
        Logger rootLogger = Logger.getLogger("");
        rootLogger.setLevel(level);

        // Replace default handler to control format
        for (var handler : rootLogger.getHandlers()) {
            rootLogger.removeHandler(handler);
        }

        ConsoleHandler handler = new ConsoleHandler();
        handler.setLevel(level);
        handler.setFormatter(new SimpleFormatter());
        rootLogger.addHandler(handler);
    }

    private static int parseMeals(String[] args) {
        if (args.length > 0) {
            try {
                int meals = Integer.parseInt(args[0]);
                if (meals <= 0) throw new IllegalArgumentException("meals must be positive");
                return meals;
            } catch (NumberFormatException e) {
                System.err.println("Invalid meals argument — using default of 3");
            }
        }
        return 3;
    }
}