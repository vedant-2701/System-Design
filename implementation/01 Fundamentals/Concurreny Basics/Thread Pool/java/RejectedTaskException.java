//RejectedTaskException.java

/**
 * Thrown when a task cannot be accepted by the pool.
 * Unchecked — callers should handle this at submission sites
 * where overload behavior matters.
 */
public class RejectedTaskException extends RuntimeException {

    public RejectedTaskException(String message) {
        super(message);
    }

    public RejectedTaskException(String message, Throwable cause) {
        super(message, cause);
    }
}