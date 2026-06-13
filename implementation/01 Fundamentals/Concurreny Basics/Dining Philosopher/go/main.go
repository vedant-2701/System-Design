package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const (
	philosopherCount = 5

	minThinkMs = 100
	maxThinkMs = 500
	minEatMs   = 100
	maxEatMs   = 300
)

// -----------------------------------------------------------------------------
// Fork
//
// A fork is a sync.Mutex — identical concept to Java's ReentrantLock.
//
// One key difference from Java:
// Go's sync.Mutex is NOT reentrant. The same goroutine cannot lock it twice
// without deadlocking itself. In Java, ReentrantLock allows the same thread
// to acquire it multiple times (with matching unlocks). This distinction
// doesn't matter here — each philosopher acquires each fork exactly once —
// but it's critical to understand when using Go mutexes in production.
//
// Another difference: Go's sync.Mutex has NO ownership enforcement.
// Any goroutine can call Unlock() on a mutex locked by another goroutine.
// This is the same risk as semaphores in Java — we must be disciplined.
// The defer pattern below enforces correct pairing.
// -----------------------------------------------------------------------------

type Fork struct {
	id  int
	mu  sync.Mutex
}

func newFork(id int) *Fork {
	return &Fork{id: id}
}

func (f *Fork) pickUp() {
	f.mu.Lock()
}

func (f *Fork) putDown() {
	f.mu.Unlock()
}

// -----------------------------------------------------------------------------
// PhilosopherState
// -----------------------------------------------------------------------------

type PhilosopherState string

const (
	StateThinking PhilosopherState = "THINKING"
	StateHungry   PhilosopherState = "HUNGRY"
	StateEating   PhilosopherState = "EATING"
	StateDone     PhilosopherState = "DONE"
)

// -----------------------------------------------------------------------------
// Philosopher
//
// In Go, a philosopher is not a struct that implements Runnable.
// Instead, the philosopher is a struct with a run() method that is
// launched as a goroutine. The goroutine IS the philosopher's thread.
//
// Goroutines vs Java Threads:
// - Goroutines are multiplexed onto OS threads by the Go runtime (M:N model)
// - Initial stack is ~2KB vs ~1MB for Java threads
// - The runtime handles scheduling — not the OS directly
// - This makes goroutines far cheaper to spawn than Java threads
//
// sync.WaitGroup replaces ExecutorService.awaitTermination —
// it lets main() wait for all goroutines to finish before exiting.
// -----------------------------------------------------------------------------

type Philosopher struct {
	id          int
	firstFork   *Fork // lower-numbered fork — always acquired first
	secondFork  *Fork // higher-numbered fork — acquired second
	totalMeals  int
	mealsEaten  int
	state       PhilosopherState
	mu          sync.Mutex // protects mealsEaten and state for safe reads
}

func newPhilosopher(id int, leftFork, rightFork *Fork, totalMeals int) *Philosopher {
	p := &Philosopher{
		id:         id,
		totalMeals: totalMeals,
		state:      StateThinking,
	}

	// Lock ordering: always assign lower-ID fork as firstFork.
	// Identical logic to Java — the deadlock prevention lives here.
	if leftFork.id < rightFork.id {
		p.firstFork  = leftFork
		p.secondFork = rightFork
	} else {
		p.firstFork  = rightFork
		p.secondFork = leftFork
	}

	return p
}

// run is launched as a goroutine. wg.Done() signals completion to main().
func (p *Philosopher) run(wg *sync.WaitGroup) {
	defer wg.Done()

	log.Printf("P%d starting — will eat %d meals\n", p.id, p.totalMeals)

	for p.getMealsEaten() < p.totalMeals {
		p.think()
		p.acquireForks()
		p.eat()
		p.releaseForks()
	}

	p.setState(StateDone)
	log.Printf("P%d finished — ate %d meals\n", p.id, p.getMealsEaten())
}

// -----------------------------------------------------------------------------
// Lifecycle steps
// -----------------------------------------------------------------------------

func (p *Philosopher) think() {
	p.setState(StateThinking)
	duration := randomBetween(minThinkMs, maxThinkMs)
	log.Printf("P%d thinking for %dms\n", p.id, duration)
	time.Sleep(time.Duration(duration) * time.Millisecond)
}

// acquireForks picks up both forks in lower-ID-first order.
//
// Note: there is NO defer here intentionally.
// defer would call putDown at the end of acquireForks itself —
// but we want the forks held across the eat() call.
// releaseForks() handles the unlock explicitly.
//
// This is a case where defer would be incorrect. Knowing when NOT
// to use defer is as important as knowing when to use it.
func (p *Philosopher) acquireForks() {
	p.setState(StateHungry)
	log.Printf("P%d hungry — waiting for Fork-%d then Fork-%d\n",
		p.id, p.firstFork.id, p.secondFork.id)

	p.firstFork.pickUp()
	log.Printf("P%d acquired Fork-%d\n", p.id, p.firstFork.id)

	p.secondFork.pickUp()
	log.Printf("P%d acquired Fork-%d — ready to eat\n", p.id, p.secondFork.id)
}

func (p *Philosopher) eat() {
	p.setState(StateEating)

	p.mu.Lock()
	p.mealsEaten++
	meals := p.mealsEaten
	p.mu.Unlock()

	duration := randomBetween(minEatMs, maxEatMs)
	log.Printf("P%d eating meal %d/%d for %dms\n",
		p.id, meals, p.totalMeals, duration)
	time.Sleep(time.Duration(duration) * time.Millisecond)
}

// releaseForks releases in reverse acquisition order — mirrors Java implementation.
func (p *Philosopher) releaseForks() {
	p.secondFork.putDown()
	p.firstFork.putDown()
	log.Printf("P%d released both forks\n", p.id)
}

// -----------------------------------------------------------------------------
// Thread-safe accessors
// Used by main() after goroutines complete — technically safe at that point,
// but explicit locking makes the intent clear and protects against future
// concurrent reads during execution.
// -----------------------------------------------------------------------------

func (p *Philosopher) getMealsEaten() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mealsEaten
}

func (p *Philosopher) setState(s PhilosopherState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = s
}

func (p *Philosopher) getState() PhilosopherState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// -----------------------------------------------------------------------------
// DiningTable
// -----------------------------------------------------------------------------

type DiningTable struct {
	forks               [philosopherCount]*Fork
	philosophers        [philosopherCount]*Philosopher
	mealsPerPhilosopher int
}

func newDiningTable(meals int) *DiningTable {
	t := &DiningTable{mealsPerPhilosopher: meals}

	for i := 0; i < philosopherCount; i++ {
		t.forks[i] = newFork(i)
	}

	for i := 0; i < philosopherCount; i++ {
		leftFork  := t.forks[i]
		rightFork := t.forks[(i+1)%philosopherCount]
		t.philosophers[i] = newPhilosopher(i, leftFork, rightFork, meals)
	}

	return t
}

// start launches all philosophers as goroutines and waits for completion.
//
// sync.WaitGroup is the Go equivalent of ExecutorService.awaitTermination.
// wg.Add(n) registers n goroutines to wait for.
// wg.Done() is called by each goroutine when it finishes (via defer).
// wg.Wait() blocks until the counter reaches zero.
//
// Unlike Java's awaitTermination, WaitGroup has no built-in timeout.
// For production use, combine with context.WithTimeout — shown in comments.
func (t *DiningTable) start() {
	log.Printf("=== Dining Table starting — %d philosophers, %d meals each ===\n",
		philosopherCount, t.mealsPerPhilosopher)

	var wg sync.WaitGroup

	for _, p := range t.philosophers {
		wg.Add(1)
		// Pass p explicitly to the goroutine closure.
		// Capturing loop variables directly in goroutines is a classic Go bug:
		// by the time the goroutine runs, the loop may have advanced and
		// all goroutines end up with the same p (the last one).
		// Passing as a parameter captures the value at launch time.
		philosopher := p
		go philosopher.run(&wg)
	}

	wg.Wait()

	log.Println("=== All philosophers finished ===")
	t.printSummary()
}

func (t *DiningTable) printSummary() {
	fmt.Println("\n=== Simulation Summary ===")
	total := 0
	for _, p := range t.philosophers {
		meals := p.getMealsEaten()
		fmt.Printf("  P%d — meals eaten: %d, final state: %s\n",
			p.id, meals, p.getState())
		total += meals
	}
	fmt.Printf("  Total meals: %d (expected: %d)\n",
		total, philosopherCount*t.mealsPerPhilosopher)
	fmt.Println("==========================")
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func randomBetween(min, max int) int {
	return min + rand.Intn(max-min+1)
}

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

func main() {
	rand.Seed(time.Now().UnixNano()) //nolint:staticcheck

	meals := 3
	table := newDiningTable(meals)
	table.start()
}