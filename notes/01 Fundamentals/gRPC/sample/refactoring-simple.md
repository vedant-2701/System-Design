# Question

> You have a production service using this message:
> ```protobuf
> message Order {
>   uint32 order_id  = 1;
>   string item_name = 2;
>   uint32 quantity  = 3;
> }
> ```
> Six months later, you realize `item_name` is wrong — orders can have multiple items. You want to replace `item_name` with a `repeated Item items` field. You also want to add a `created_at` timestamp.
> What are the safe steps to do this migration without breaking existing clients that are still sending the old message format? What field numbers should the new fields use, and what should you do with field numbers 2?

---

# Answer

```protobuf
import "google/protobuf/timestamp.proto";

message Item {
  string name     = 1;
  uint32 quantity = 2;
}

message Order {
  uint32   order_id   = 1;
  reserved 2;
  reserved "item_name";
  uint32   quantity   = 3;
  repeated Item items = 4;
  google.protobuf.Timestamp created_at = 5;
}
```

* Using raw `int64` (Unix epoch milliseconds or seconds) is common in practice, but you should know the distinction — `Timestamp` gives you seconds + nanoseconds precision and is interoperable with language-native time types via generated code. For an interview or production system, using the well-known type is the more correct choice.

---

**Note:**
> Old clients sending `Order` with field 2 (`item_name`) will have that data silently ignored by the new server. The new server doesn't know field 2 anymore — it's reserved. The bytes arrive, the deserializer sees field 2, finds no mapping in the current schema, and discards it.
> 
> This is safe. The old client doesn't crash. The new server doesn't crash. Data is lost for that field, which is acceptable because you're intentionally deprecating it.
>
> But here's a follow-up implication: new clients talking to old servers. A new client sends field 4 (`items`) and field 5 (`created_at`). The old server sees fields it doesn't know — 4 and 5 — and discards them. The old server still processes `order_id` and `quantity` fine.
>
> This bidirectional tolerance is what makes Protobuf schema evolution safe in practice — as long as you never reuse field numbers.