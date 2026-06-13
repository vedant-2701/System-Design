# Go Syntax Guide - For Python/Java/C Developers

## 1. Basic Program Structure

```go
package main

import (
    "fmt"
)

func main() {
    fmt.Println("Hello, World!")
}
```

**Key differences from what you know:**
- **`package main`**: Every Go file belongs to a package. `main` is special—it's where execution starts
- **`import`**: Similar to Python's `import` or Java's `import`
- **`func`**: Function keyword (like `def` in Python)
- No classes or OOP by default (more on this later)

---

## 2. Variables & Constants

### Variable Declaration

**Multiple ways to declare variables:**

```go
// Explicit type declaration
var name string = "Alice"
var age int = 30

// Type inference (Go figures out the type)
var city = "Mumbai"  // Go sees it's a string

// Short declaration (only inside functions, most common)
message := "Hello"   // Creates and assigns in one line
count := 42

// Multiple declarations
var (
    firstName = "John"
    lastName  = "Doe"
    isActive  = true
)

// Multiple assignment
a, b, c := 1, 2, 3
```

**Comparison to Python/Java:**
```python
# Python
name = "Alice"  # No type needed
age = 30

# Java
String name = "Alice";
int age = 30;
```

```go
// Go - similar to Java but more flexible
var name string = "Alice"  // Explicit
name := "Alice"             // Short form (preferred in Go)
```

### Constants

```go
const (
    MaxPlayers = 100
    MinPlayers = 2
    GameName   = "Chess"
)
```

**Key difference:** Constants are compile-time values. You can't use `:=` for constants.

---

## 3. Data Types

```go
// Numbers
var age int = 25              // Default int size (32/64 bit depending on OS)
var price float64 = 19.99     // Floating point (always use float64)
var smallNum int8 = 127       // Specific size int
var largeNum uint64 = 999999  // Unsigned (no negatives)

// Strings
var text string = "Hello"
var char rune = 'A'           // Single character (Unicode)
var byte byte = 65            // Single byte

// Booleans
var isActive bool = true

// Collections
var numbers []int = []int{1, 2, 3}        // Slice (dynamic array)
var fixed [5]int = [5]int{1, 2, 3, 0, 0} // Array (fixed size)
var person map[string]string = map[string]string{
    "name": "Alice",
    "city": "Mumbai",
}

// Comparison to Python
# Python
age = 25          # Any type
price = 19.99
text = "Hello"
numbers = [1, 2, 3]
person = {"name": "Alice", "city": "Mumbai"}
```

---

## 4. Functions

### Basic Function

```go
func greet(name string) string {
    return "Hello, " + name
}

// Call it
result := greet("Alice")
fmt.Println(result)  // Output: Hello, Alice
```

### Multiple Return Values (Go feature!)

```go
// This is powerful in Go - no need for exceptions or tuples
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("cannot divide by zero")
    }
    return a / b, nil
}

// Usage
result, err := divide(10, 2)
if err != nil {
    fmt.Println("Error:", err)
} else {
    fmt.Println("Result:", result)  // 5
}
```

**Comparison to Python:**
```python
# Python - use tuple
def divide(a, b):
    if b == 0:
        raise ValueError("cannot divide by zero")
    return a / b, None  # Not idiomatic

# Go is cleaner for error handling
```

### Named Return Values

```go
func swap(a, b int) (first, second int) {
    return b, a  // Returns in order: first=b, second=a
}

x, y := swap(5, 10)
fmt.Println(x, y)  // Output: 10 5
```

### Variadic Functions (Variable Arguments)

```go
func sum(numbers ...int) int {
    total := 0
    for _, num := range numbers {
        total += num
    }
    return total
}

result := sum(1, 2, 3, 4, 5)
fmt.Println(result)  // Output: 15
```

**Similar to Python:**
```python
def sum(*numbers):
    return sum(numbers)
```

---

## 5. Control Flow

### If/Else

```go
age := 20

if age < 18 {
    fmt.Println("Minor")
} else if age < 65 {
    fmt.Println("Adult")
} else {
    fmt.Println("Senior")
}

// You can declare variables in the if condition
if age := getUserAge(); age > 18 {
    fmt.Println("Can vote")
    // age is only available in this block
}
```

### Switch

```go
day := "Monday"

switch day {
case "Monday":
    fmt.Println("Start of week")
case "Friday":
    fmt.Println("Almost weekend")
default:
    fmt.Println("Other day")
}

// Type switch (useful with interfaces)
switch v := something.(type) {
case int:
    fmt.Println("It's an int:", v)
case string:
    fmt.Println("It's a string:", v)
default:
    fmt.Println("Unknown type")
}
```

### Loops

```go
// For loop (Go only has 'for', not 'while')
for i := 0; i < 5; i++ {
    fmt.Println(i)
}

// While style
count := 0
for count < 5 {
    fmt.Println(count)
    count++
}

// Infinite loop
for {
    fmt.Println("Forever")
    break  // Must break manually
}

// Range (iterate over collections)
numbers := []int{1, 2, 3, 4, 5}
for index, value := range numbers {
    fmt.Println(index, value)
}

// If you only want the value
for _, value := range numbers {
    fmt.Println(value)
}

// If you only want the index
for index := range numbers {
    fmt.Println(index)
}

// Iterating over strings
for index, char := range "Hello" {
    fmt.Println(index, string(char))  // char is a rune
}

// Iterating over maps
person := map[string]string{"name": "Alice", "city": "Mumbai"}
for key, value := range person {
    fmt.Println(key, value)
}
```

**Comparison to Python:**
```python
# Python has while and for
for i in range(5):
    print(i)

while count < 5:
    print(count)

for index, value in enumerate(numbers):
    print(index, value)

for key, value in person.items():
    print(key, value)
```

---

## 6. Arrays & Slices

**Arrays are fixed-size, slices are dynamic (like Python lists)**

### Arrays (Fixed Size)

```go
// Declare array of 5 integers
var numbers [5]int = [5]int{1, 2, 3, 4, 5}

// Array literal
colors := [3]string{"red", "green", "blue"}

// Access
fmt.Println(numbers[0])  // 1
fmt.Println(len(numbers))  // 5

// Modify
numbers[0] = 10
```

### Slices (Dynamic - More Common)

```go
// Create slice
var numbers []int = []int{1, 2, 3, 4, 5}
numbers := []int{1, 2, 3, 4, 5}  // Short form

// From array
arr := [5]int{1, 2, 3, 4, 5}
slice := arr[1:4]  // Elements at index 1, 2, 3 (not 4)
fmt.Println(slice)  // [2 3 4]

// Slicing operations
slice := numbers[1:]      // From index 1 to end
slice := numbers[:3]      // From start to index 2 (not 3)
slice := numbers[1:3]     // Index 1 and 2

// Append (creates new slice if needed)
slice := []int{1, 2, 3}
slice = append(slice, 4)      // Add one element
slice = append(slice, 5, 6)   // Add multiple elements

// Length and capacity
slice := make([]int, 3, 5)  // Length 3, capacity 5
fmt.Println(len(slice))      // 3
fmt.Println(cap(slice))      // 5

// Create empty slice
var empty []int
empty = append(empty, 1)

// Using make()
slice := make([]int, 5)      // Length 5, all zeros
slice := make([]int, 0, 10)  // Length 0, capacity 10
```

---

## 7. Maps (Dictionaries/Hash Maps)

```go
// Create map
person := map[string]string{
    "name": "Alice",
    "city": "Mumbai",
}

// Access
fmt.Println(person["name"])  // Alice

// Add/modify
person["age"] = "30"

// Check if key exists
value, exists := person["age"]
if exists {
    fmt.Println("Age:", value)
}

// Delete key
delete(person, "age")

// Empty map
var empty map[string]int
empty = make(map[string]int)  // Must use make() before adding

// Iterate
for key, value := range person {
    fmt.Println(key, value)
}
```

**Comparison to Python:**
```python
person = {"name": "Alice", "city": "Mumbai"}
if "age" in person:
    print(person["age"])
del person["age"]
```

---

## 8. Structs (Like Classes)

Go doesn't have classes, but **structs** + **methods** give similar functionality.

```go
// Define a struct
type Person struct {
    Name string
    Age  int
    City string
}

// Create instance
alice := Person{
    Name: "Alice",
    Age:  30,
    City: "Mumbai",
}

// Access fields
fmt.Println(alice.Name)  // Alice

// Modify fields
alice.Age = 31

// Shorthand (positional)
bob := Person{"Bob", 25, "Delhi"}

// Empty struct
charlie := Person{}  // All fields are zero values (empty string, 0, etc)

// Pointers to structs
var ptr *Person = &alice
fmt.Println(ptr.Name)  // Alice (Go auto-dereferences)
ptr.Age = 32           // Works on pointer
```

**Comparison to Java:**
```java
// Java
class Person {
    public String name;
    public int age;
    public String city;
}
Person alice = new Person();
alice.name = "Alice";
```

```go
// Go
type Person struct {
    Name string
    Age  int
    City string
}
alice := Person{Name: "Alice", Age: 30, City: "Mumbai"}
```

---

## 9. Methods (Functions on Structs)

```go
type Person struct {
    Name string
    Age  int
}

// Method with receiver (like 'this' or 'self')
func (p Person) Greet() string {
    return "Hello, I'm " + p.Name
}

// Method that modifies (needs pointer receiver)
func (p *Person) Birthday() {
    p.Age++
}

// Usage
alice := Person{Name: "Alice", Age: 30}
fmt.Println(alice.Greet())  // Hello, I'm Alice

alice.Birthday()  // Increments age
fmt.Println(alice.Age)  // 31
```

**Comparison to Java/Python:**
```java
// Java
class Person {
    public String name;
    public int age;
    
    public String greet() {
        return "Hello, I'm " + name;
    }
}
```

```go
// Go - similar idea
type Person struct {
    Name string
    Age  int
}

func (p Person) Greet() string {
    return "Hello, I'm " + p.Name
}
```

---

## 10. Interfaces

Interfaces define what methods a type must have. Very powerful in Go.

```go
// Define interface
type Animal interface {
    Speak() string
    Move()
}

// Dog implements Animal
type Dog struct {
    Name string
}

func (d Dog) Speak() string {
    return "Woof!"
}

func (d Dog) Move() {
    fmt.Println("Dog runs")
}

// Cat implements Animal
type Cat struct {
    Name string
}

func (c Cat) Speak() string {
    return "Meow!"
}

func (c Cat) Move() {
    fmt.Println("Cat walks")
}

// Function that accepts any Animal
func MakeSound(animal Animal) {
    fmt.Println(animal.Speak())
    animal.Move()
}

// Usage
dog := Dog{Name: "Buddy"}
cat := Cat{Name: "Whiskers"}

MakeSound(dog)  // Woof! Dog runs
MakeSound(cat)  // Meow! Cat walks
```

**Key difference from Java:**
- In Java, you explicitly say `class Dog implements Animal`
- In Go, **implicit**—if Dog has the methods, it's an Animal (duck typing)

---

## 11. Error Handling

Go uses a simple error convention instead of exceptions.

```go
// Creating errors
import "errors"

func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

// Using errors
result, err := divide(10, 0)
if err != nil {
    fmt.Println("Error:", err)
} else {
    fmt.Println("Result:", result)
}

// With fmt.Errorf (formatted errors)
func getUserAge(id int) (int, error) {
    if id < 0 {
        return 0, fmt.Errorf("invalid user ID: %d", id)
    }
    return 25, nil
}
```

**Comparison to Python/Java:**
```python
# Python uses try/except
try:
    result = divide(10, 0)
except ZeroDivisionError as e:
    print("Error:", e)

# Go is explicit
result, err := divide(10, 0)
if err != nil {
    fmt.Println("Error:", err)
}
```

---

## 12. Defer, Panic, Recover

```go
// Defer: execute at function end
func readFile(filename string) {
    file := openFile(filename)
    defer file.Close()  // Runs when function exits
    
    // Read file...
}

// Multiple defers (last-in-first-out)
defer fmt.Println("3")
defer fmt.Println("2")
defer fmt.Println("1")
// Output: 1, 2, 3

// Panic: like throwing an exception
func riskyFunction() {
    panic("Something went wrong!")
}

// Recover: catch panic
func SafeFunction() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("Recovered:", r)
        }
    }()
    
    riskyFunction()  // Won't crash program
}
```

---

## 13. Pointers

Go has pointers, but simpler than C.

```go
x := 10
ptr := &x      // Address of x
fmt.Println(*ptr)  // Dereference: 10

*ptr = 20      // Change value at address
fmt.Println(x)  // 20

// Functions with pointers
func increment(p *int) {
    *p++
}

num := 5
increment(&num)
fmt.Println(num)  // 6

// Difference: slices and maps are already reference types
slice := []int{1, 2, 3}
slice2 := slice  // slice2 points to same underlying array
slice[0] = 999
fmt.Println(slice2[0])  // 999
```

---

## 14. Goroutines (Concurrency)

Go's killer feature for concurrent programming.

```go
import "time"

func task(name string) {
    for i := 1; i <= 3; i++ {
        fmt.Println(name, i)
        time.Sleep(1 * time.Second)
    }
}

// Sequential (slow)
task("A")
task("B")

// Concurrent (fast, with goroutines)
go task("A")
go task("B")
time.Sleep(5 * time.Second)  // Wait for goroutines

// With channels (communicate between goroutines)
messages := make(chan string)

go func() {
    messages <- "Hello"
}()

go func() {
    messages <- "World"
}()

msg1 := <-messages
msg2 := <-messages
fmt.Println(msg1, msg2)
```

---

## 15. Package Organization

```
myproject/
├── main.go          // package main
├── utils/
│   └── helper.go    // package utils
└── models/
    └── user.go      // package models
```

**main.go:**
```go
package main

import (
    "myproject/utils"
    "myproject/models"
)

func main() {
    utils.SomeFunction()  // Exported (capital letter)
}
```

**utils/helper.go:**
```go
package utils

// Exported (capital letter)
func SomeFunction() {
    // ...
}

// Not exported (lowercase)
func privateHelper() {
    // ...
}
```

**Key rule:** Capital letters = exported (public), lowercase = private

---

## 16. Common Patterns

### The Error Check Pattern

```go
// Very common in Go
file, err := os.Open("test.txt")
if err != nil {
    return err  // or handle error
}
defer file.Close()
```

### The Blank Identifier

```go
// Ignore a return value
_, err := someFunction()  // Ignore first return
if err != nil {
    // handle error
}
```

### Type Assertion

```go
var x interface{} = "hello"

str := x.(string)           // Assert x is string
str, ok := x.(string)       // Safe assertion
if ok {
    fmt.Println(str)
}
```

---

## Quick Cheat Sheet

| Concept | Go | Python | Java |
|---------|----|---------|----|
| Variable | `x := 5` | `x = 5` | `int x = 5;` |
| Constant | `const X = 5` | N/A | `final int X = 5;` |
| Function | `func add(a, b int) int { return a+b }` | `def add(a, b): return a+b` | `public int add(int a, int b) { return a+b; }` |
| Array | `[5]int{1,2,3}` | `[1,2,3]` | `int[] arr = {1,2,3};` |
| Dictionary | `map[string]int` | `dict` | `HashMap<String, Integer>` |
| Loop | `for i := 0; i < 5; i++` | `for i in range(5):` | `for(int i=0;i<5;i++)` |
| Error handling | `if err != nil` | `try/except` | `try/catch` |

---

## Next Steps to Practice

1. **Install Go:** `https://golang.org/doc/install`
2. **Try online:** `https://go.dev/play/`
3. **Write a simple program:**
   - Read user input
   - Do some calculations
   - Handle errors
4. **Build something:** CLI tool, HTTP server, data processor
5. **Learn concurrency:** Goroutines and channels are Go's strength

Go is designed to be simple and explicit. You'll notice it values clarity over cleverness. Coming from Python/Java, you'll pick it up quickly!
