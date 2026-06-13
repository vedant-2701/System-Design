package main

import (
	"fmt"
	"errors"
)

// ============================================
// EXAMPLE 1: Basic Variables and Functions
// ============================================

func example1_BasicVariables() {
	fmt.Println("\n=== EXAMPLE 1: Variables ===")
	
	// Different ways to declare variables
	var name string = "Alice"          // Explicit type
	age := 30                          // Type inference
	var city = "Mumbai"                // Type inference with var
	
	fmt.Printf("Name: %s, Age: %d, City: %s\n", name, age, city)
	
	// Multiple variable declaration
	var (
		firstName = "John"
		lastName  = "Doe"
		isActive  = true
	)
	fmt.Printf("Person: %s %s (Active: %v)\n", firstName, lastName, isActive)
}

// ============================================
// EXAMPLE 2: Functions with Multiple Returns
// ============================================

func divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("cannot divide by zero")
	}
	return a / b, nil
}

func example2_Functions() {
	fmt.Println("\n=== EXAMPLE 2: Functions with Error Handling ===")
	
	// Successful division
	result, err := divide(10, 2)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("10 / 2 =", result)
	}
	
	// Division by zero
	result, err = divide(10, 0)
	if err != nil {
		fmt.Println("Error:", err)  // Error: cannot divide by zero
	}
}

// ============================================
// EXAMPLE 3: Structs and Methods
// ============================================

type Person struct {
	Name string
	Age  int
	City string
}

// Method with value receiver
func (p Person) Greet() string {
	return fmt.Sprintf("Hello, I'm %s from %s", p.Name, p.City)
}

// Method with pointer receiver (can modify)
func (p *Person) HaveBirthday() {
	p.Age++
}

func example3_StructsAndMethods() {
	fmt.Println("\n=== EXAMPLE 3: Structs and Methods ===")
	
	// Create a Person
	alice := Person{
		Name: "Alice",
		Age:  30,
		City: "Mumbai",
	}
	
	fmt.Println(alice.Greet())  // Hello, I'm Alice from Mumbai
	fmt.Println("Age before birthday:", alice.Age)
	
	alice.HaveBirthday()
	fmt.Println("Age after birthday:", alice.Age)
}

// ============================================
// EXAMPLE 4: Interfaces
// ============================================

type Animal interface {
	Speak() string
	Move()
}

type Dog struct {
	Name string
}

func (d Dog) Speak() string {
	return "Woof!"
}

func (d Dog) Move() {
	fmt.Println("Dog runs with tail wagging")
}

type Cat struct {
	Name string
}

func (c Cat) Speak() string {
	return "Meow!"
}

func (c Cat) Move() {
	fmt.Println("Cat walks gracefully")
}

func MakeSound(animal Animal) {
	fmt.Printf("%s says: %s\n", animal.(interface{}).(*Dog).Name, animal.Speak())
}

func example4_Interfaces() {
	fmt.Println("\n=== EXAMPLE 4: Interfaces ===")
	
	dog := Dog{Name: "Buddy"}
	cat := Cat{Name: "Whiskers"}
	
	// Both implement Animal interface
	animals := []Animal{dog, cat}
	
	for _, animal := range animals {
		fmt.Println(animal.Speak())
		animal.Move()
	}
}

// ============================================
// EXAMPLE 5: Slices and Maps
// ============================================

func example5_SlicesAndMaps() {
	fmt.Println("\n=== EXAMPLE 5: Slices and Maps ===")
	
	// Slices (dynamic arrays)
	numbers := []int{1, 2, 3, 4, 5}
	fmt.Println("Original slice:", numbers)
	
	// Slice operations
	fmt.Println("numbers[1:4]:", numbers[1:4])  // [2 3 4]
	
	// Append
	numbers = append(numbers, 6, 7)
	fmt.Println("After append:", numbers)
	
	// Maps (dictionaries)
	person := map[string]string{
		"name": "Alice",
		"city": "Mumbai",
		"job":  "Engineer",
	}
	
	fmt.Println("\nMap:", person)
	fmt.Println("person['name']:", person["name"])
	
	// Check if key exists
	job, exists := person["job"]
	if exists {
		fmt.Println("Job:", job)
	}
	
	// Delete from map
	delete(person, "job")
	fmt.Println("After delete:", person)
}

// ============================================
// EXAMPLE 6: Control Flow
// ============================================

func example6_ControlFlow() {
	fmt.Println("\n=== EXAMPLE 6: Control Flow ===")
	
	// If/else with variable declaration
	age := 25
	if age < 18 {
		fmt.Println("Minor")
	} else if age < 65 {
		fmt.Println("Adult")
	} else {
		fmt.Println("Senior")
	}
	
	// Switch
	day := "Monday"
	switch day {
	case "Monday":
		fmt.Println("Start of week")
	case "Friday":
		fmt.Println("Almost weekend")
	default:
		fmt.Println("Other day")
	}
	
	// For loop variations
	fmt.Println("\nLoop variations:")
	
	// Traditional for loop
	for i := 0; i < 3; i++ {
		fmt.Printf("i = %d\n", i)
	}
	
	// Range over slice
	fruits := []string{"apple", "banana", "cherry"}
	for index, fruit := range fruits {
		fmt.Printf("Index %d: %s\n", index, fruit)
	}
}

// ============================================
// EXAMPLE 7: Defer and Cleanup
// ============================================

func example7_Defer() {
	fmt.Println("\n=== EXAMPLE 7: Defer ===")
	
	defer fmt.Println("3. This runs last (deferred)")
	fmt.Println("1. This runs first")
	
	defer fmt.Println("2. This runs second (deferred)")
	
	fmt.Println("1.5. Still in the middle")
}

// ============================================
// EXAMPLE 8: Pointers
// ============================================

func example8_Pointers() {
	fmt.Println("\n=== EXAMPLE 8: Pointers ===")
	
	x := 10
	ptr := &x  // Address of x
	
	fmt.Printf("x = %d\n", x)
	fmt.Printf("ptr points to address: %p\n", ptr)
	fmt.Printf("*ptr (dereferenced) = %d\n", *ptr)
	
	// Modify through pointer
	*ptr = 20
	fmt.Printf("After *ptr = 20, x = %d\n", x)
	
	// Slices are reference types
	slice1 := []int{1, 2, 3}
	slice2 := slice1
	slice2[0] = 999
	
	fmt.Printf("slice1[0] = %d (changed by slice2!)\n", slice1[0])
}

// ============================================
// EXAMPLE 9: Type Conversions
// ============================================

func example9_TypeConversions() {
	fmt.Println("\n=== EXAMPLE 9: Type Conversions ===")
	
	// Explicit type conversion
	var x int = 10
	var y float64 = float64(x)
	
	fmt.Printf("int %d -> float64 %f\n", x, y)
	
	// String conversion
	str := "Hello"
	for _, char := range str {
		fmt.Printf("Char: %c (rune)\n", char)
		break  // Just show first one
	}
	
	// String to int
	var num = 42
	str = fmt.Sprintf("%d", num)  // int to string
	fmt.Printf("Converted: %s (string)\n", str)
}

// ============================================
// EXAMPLE 10: Variadic Functions
// ============================================

func sum(numbers ...int) int {
	total := 0
	for _, num := range numbers {
		total += num
	}
	return total
}

func example10_Variadic() {
	fmt.Println("\n=== EXAMPLE 10: Variadic Functions ===")
	
	result := sum(1, 2, 3, 4, 5)
	fmt.Println("sum(1,2,3,4,5) =", result)
	
	// With slice
	numbers := []int{10, 20, 30}
	result = sum(numbers...)  // ... unpacks the slice
	fmt.Println("sum(10,20,30) =", result)
}

// ============================================
// Main Function - Runs All Examples
// ============================================

func main() {
	fmt.Println("================================")
	fmt.Println("Go Syntax Examples")
	fmt.Println("================================")
	
	example1_BasicVariables()
	example2_Functions()
	example3_StructsAndMethods()
	example4_Interfaces()
	example5_SlicesAndMaps()
	example6_ControlFlow()
	example7_Defer()
	example8_Pointers()
	example9_TypeConversions()
	example10_Variadic()
	
	fmt.Println("\n================================")
	fmt.Println("All examples completed!")
	fmt.Println("================================")
}
