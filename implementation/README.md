# System Design Implementation Test Guide

The directory contains Java and Go implementations of system design.

---

## 🚀 The Automated Way: Using `run-tests.ps1`

We provide a PowerShell script `run-tests.ps1` at the root that automates compilation, execution, and cleanup.

### Prerequisites
1. **Java Development Kit (JDK)**: Ensure `javac` and `java` are in your system PATH (JDK 8 or higher).
2. **Execution Policy**: Enable running PowerShell scripts if you haven't already:
   ```powershell
   Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
   ```

### Running Tests for a Specific File
To compile all dependencies and run tests defined in a single file, pass the test file path:
```powershell
.\run-tests.ps1 -Target ".\01 Fundamentals\Concurreny Basics\LRU Cache\java\LRUCacheTest.java"
```

### Running All Tests in a Directory
To compile and run all test files in a folder, pass the folder path (the script scans the classpath automatically):
```powershell
.\run-tests.ps1 -Target ".\01 Fundamentals\Concurreny Basics\LRU Cache\java"
```

### Script Execution Flow
1. Resolves relative/absolute target paths.
2. Changes the location to the target directory.
3. Compiles all `.java` files in the folder using `javac` with `lib/junit-platform-console-standalone-6.1.0.jar` on the classpath.
4. Executes JUnit console runner against the targeted test class or scans the classpath.
5. Cleans up all generated `.class` files (even if compilation or tests fail).
6. Returns you to your original directory.

---

## 🛠️ The Manual Way

If you prefer to compile and run tests manually, follow these steps. 

### Commands (Running from target directory)
1. **Navigate to the directory** containing the source and test files:
   ```powershell
   cd ".\01 Fundamentals\Concurreny Basics\LRU Cache\java"
   ```
2. **Compile** all Java files, referencing the JUnit JAR (located in `lib` 4 directories up):
   ```powershell
   javac -cp ".;..\..\..\..\lib\junit-platform-console-standalone-6.1.0.jar" *.java
   ```
3. **Execute** tests using the console launcher:
   ```powershell
   java -jar "..\..\..\..\lib\junit-platform-console-standalone-6.1.0.jar" execute --class-path . --select-class LRUCacheTest
   ```
4. **Clean up** generated class files to keep the repository clean:
   ```powershell
   Remove-Item *.class
   ```

### ⚠️ Common Pitfalls: Why did I get "0 tests found"?
If you compile and run from the **root** folder, you might encounter issues:
1. **Classpath Mismatch**: If you run:
   ```powershell
   java -jar .\lib\junit-platform-console-standalone-6.1.0.jar execute --class-path . --select-class LRUCacheTest
   ```
   from the root, JUnit search only looks for `LRUCacheTest` class directly under the root (`.`). Since the compiled class files are inside the nested subdirectories (e.g. `.\01 Fundamentals\Concurreny Basics\LRU Cache\java\`), JUnit cannot load the class and reports `0 tests found`.
2. **Relative Path to JAR**: When executing commands nested deep in the directories, the relative path to the JUnit JAR must be correctly adjusted (e.g., `..\..\..\..\lib\junit-platform-console-standalone-6.1.0.jar` for 4 levels up to the root, then into the `lib` folder).

---

## 🐹 Running Go Files & Tests

The Go implementation is structured as a library package (`package lrucache`) with associated unit tests (`lrucache_test.go`).

### Why did `go run .\lrucache.go` fail?
In Go, `go run` compiles and runs files belonging to the `main` package (which defines an entry point `func main()`). Because our files use `package lrucache`, they form a library package rather than an executable application.

### Commands (Running tests)
1. **Navigate to the Go directory**:
   ```powershell
   cd ".\01 Fundamentals\Concurreny Basics\LRU Cache\go"
   ```
2. **Run all tests**:
   ```powershell
   go test
   ```
3. **Run tests in verbose mode** (showing individual test names and logs):
   ```powershell
   go test -v
   ```
4. **Run a specific test**:
   ```powershell
   go test -v -run TestConcurrentPuts_DoNotCorruptCache
   ```

---

## 💡 Modern & Better Alternatives

While running standalone JARs via script is lightweight, here are standard professional alternatives:

### 1. Standard Java Build Tools (Maven / Gradle)
In professional projects, build tools manage dependencies, compilation, and testing out-of-the-box.
- **Maven**: Run `mvn test`
  - Requires a `pom.xml` file.
  - Automatically downloads JUnit libraries and builds the classpaths.
- **Gradle**: Run `gradle test`
  - Requires a `build.gradle` file.
  - Very fast compilation and test runs due to build caching.

### 2. IDE Extensions (VS Code Integration)
If using VS Code, you can run tests with one click directly from the UI:
1. Install the **Extension Pack for Java** and **Test Runner for Java** extensions.
2. The IDE will automatically scan and detect JUnit test cases.
3. Click the **Run Test** (Play icon) overlay directly above your test methods/classes.
