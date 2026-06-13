// LRUCache.java
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.locks.ReentrantLock;

public class LRUCache<K, V> implements Cache<K, V> {

    private final int capacity;
    private final Map<K, Node<K, V>> cacheMap;
    private final Node<K, V> head; // sentinel — most recently used end
    private final Node<K, V> tail; // sentinel — least recently used end
    private final ReentrantLock lock;

    public LRUCache(int capacity) {
        if (capacity <= 0) {
            throw new IllegalArgumentException(
                "Cache capacity must be positive, got: " + capacity
            );
        }
        this.capacity = capacity;
        this.cacheMap = new HashMap<>(capacity);
        this.lock = new ReentrantLock();

        // Sentinel nodes eliminate null checks on every insert/remove.
        // head.next is always the MRU node.
        // tail.prev is always the LRU node.
        this.head = new Node<>(null, null);
        this.tail = new Node<>(null, null);
        head.next = tail;
        tail.prev = head;
    }

    @Override
    public V get(K key) {
        lock.lock();
        try {
            Node<K, V> node = cacheMap.get(key);
            if (node == null) {
                return null; // cache miss
            }
            moveToHead(node); // mark as most recently used
            return node.value;
        } finally {
            lock.unlock(); // always releases — even if exception thrown
        }
    }

    @Override
    public void put(K key, V value) {
        if (key == null) {
            throw new IllegalArgumentException("Cache key must not be null");
        }
        lock.lock();
        try {
            Node<K, V> existingNode = cacheMap.get(key);

            if (existingNode != null) {
                // Key exists — update value in-place, move to head.
                // No new node allocation, no eviction needed.
                existingNode.value = value;
                moveToHead(existingNode);
                return;
            }

            // New key — create node and insert at head (MRU position)
            Node<K, V> newNode = new Node<>(key, value);
            cacheMap.put(key, newNode);
            insertAtHead(newNode);

            // Evict LRU entry if over capacity.
            // Check + evict is atomic inside the lock — no race condition.
            if (cacheMap.size() > capacity) {
                Node<K, V> lruNode = tail.prev;
                removeNode(lruNode);
                cacheMap.remove(lruNode.key); // key stored on node for this
            }
        } finally {
            lock.unlock();
        }
    }

    @Override
    public int size() {
        lock.lock();
        try {
            return cacheMap.size();
        } finally {
            lock.unlock();
        }
    }

    @Override
    public void clear() {
        lock.lock();
        try {
            cacheMap.clear();
            head.next = tail;
            tail.prev = head;
        } finally {
            lock.unlock();
        }
    }

    // --- Private list operations ---
    // These are never called outside the lock. No synchronization needed here.

    private void insertAtHead(Node<K, V> node) {
        node.prev = head;
        node.next = head.next;
        head.next.prev = node;
        head.next = node;
    }

    private void removeNode(Node<K, V> node) {
        node.prev.next = node.next;
        node.next.prev = node.prev;
        // prev/next pointers left intact intentionally —
        // node is about to be GC'd, no need to null them out
    }

    private void moveToHead(Node<K, V> node) {
        removeNode(node);
        insertAtHead(node);
    }
}