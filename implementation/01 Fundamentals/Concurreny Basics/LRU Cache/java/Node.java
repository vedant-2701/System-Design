// Node.java
class Node<K,V> {
    K key;
    V value;
    Node<K,V> prev;
    Node<K,V> next;

    Node(K key, V value) {
        this.key = key;
        this.value = value;
    }
}