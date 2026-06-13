# System Design Mastery Roadmap — Progress Tracker

---

## Phase 1 — Foundations

### Operating Systems
- [ ] Processes vs Threads
- [ ] Context Switching
- [ ] System Calls
- [ ] Memory Layout (Stack vs Heap)
- [ ] Virtual Memory & Paging
- [ ] File Descriptors
- [ ] Signals
- [ ] Process Scheduling

### Concurrency Basics
- [ ] Mutex, Semaphore, Spinlock
- [ ] Deadlock, Livelock, Starvation
- [ ] Thread Pools
- [ ] Async/Await vs Threads vs Event Loop
- [ ] Memory Visibility & Happens-Before
- [ ] Compare-and-Swap (CAS)
- [ ] Amdahl's Law

### Linux Basics
- [ ] File System Structure
- [ ] Permissions & Ownership
- [ ] Process Management (ps, kill, top)
- [ ] Networking Tools (netstat, ss, tcpdump)
- [ ] Systemd & Services
- [ ] Shell Scripting Essentials
- [ ] cron Jobs
- [ ] Log Inspection (/var/log, journalctl)

### Networking & TCP/IP
- [ ] OSI Model
- [ ] IP Addressing, Subnets, CIDR
- [ ] TCP 3-Way Handshake
- [ ] UDP vs TCP
- [ ] Ports and Sockets
- [ ] NAT & Routing
- [ ] ICMP, ping, traceroute
- [ ] TCP Congestion Control
- [ ] TIME_WAIT State

### HTTP/HTTPS & TLS/SSL
- [ ] HTTP/1.1 vs HTTP/2 vs HTTP/3
- [ ] Request/Response Lifecycle
- [ ] Status Codes (1xx–5xx)
- [ ] Headers (Auth, Caching, Content Negotiation)
- [ ] Cookies & Sessions
- [ ] TLS Handshake
- [ ] Certificate Chain & CA
- [ ] HSTS
- [ ] CORS

### DNS
- [ ] DNS Resolution Chain
- [ ] Record Types (A, AAAA, CNAME, MX, TXT)
- [ ] TTL and Caching
- [ ] Recursive vs Authoritative Resolvers
- [ ] DNS Propagation
- [ ] GeoDNS
- [ ] DNS-Based Load Balancing
- [ ] DNSSEC Basics

### REST APIs
- [ ] REST Constraints & Richardson Maturity Model
- [ ] Idempotency of HTTP Verbs
- [ ] API Versioning Strategies
- [ ] Pagination (Cursor vs Offset)
- [ ] HATEOAS
- [ ] OpenAPI / Swagger
- [ ] Error Response Design
- [ ] HTTP Caching Headers (ETag, Cache-Control)

### gRPC
- [ ] Protocol Buffers (protobuf)
- [ ] Service Definition (.proto files)
- [ ] Unary vs Streaming RPCs
- [ ] gRPC vs REST Tradeoffs
- [ ] Interceptors
- [ ] Load Balancing with gRPC
- [ ] Health Checks
- [ ] gRPC-Gateway

### WebSockets
- [ ] WebSocket Handshake (HTTP Upgrade)
- [ ] Full-Duplex Communication
- [ ] Heartbeats / Ping-Pong
- [ ] Scaling WebSocket Servers
- [ ] Socket.IO vs Raw WebSocket
- [ ] Long Polling vs SSE vs WebSocket

---

## Phase 2 — Programming & OOP

### OOP Fundamentals
- [ ] Encapsulation, Abstraction, Inheritance, Polymorphism
- [ ] Composition over Inheritance
- [ ] Interface vs Abstract Class
- [ ] Method Overloading vs Overriding
- [ ] Static vs Instance Members
- [ ] Object Lifecycle

### SOLID Principles
- [ ] Single Responsibility Principle (SRP)
- [ ] Open/Closed Principle (OCP)
- [ ] Liskov Substitution Principle (LSP)
- [ ] Interface Segregation Principle (ISP)
- [ ] Dependency Inversion Principle (DIP)

### Design Principles
- [ ] DRY (Don't Repeat Yourself)
- [ ] KISS (Keep It Simple, Stupid)
- [ ] YAGNI (You Ain't Gonna Need It)
- [ ] Rule of Three for DRY
- [ ] Avoiding Premature Abstraction

### Clean Code
- [ ] Meaningful Naming
- [ ] Small Functions / Single Level of Abstraction
- [ ] Avoiding Magic Numbers
- [ ] Comments vs Self-Documenting Code
- [ ] Guard Clauses
- [ ] Error Handling Patterns

### Dependency Injection
- [ ] Constructor vs Setter vs Interface Injection
- [ ] DI Containers (Spring, Guice)
- [ ] Manual DI
- [ ] Service Locator Anti-Pattern
- [ ] DI and Testability

### UML
- [ ] Class Diagram
- [ ] Sequence Diagram
- [ ] Use Case Diagram
- [ ] Activity Diagram
- [ ] Component Diagram

### Refactoring
- [ ] Code Smells Catalog
- [ ] Extract Method / Class
- [ ] Replace Conditional with Polymorphism
- [ ] Strangler Fig Pattern
- [ ] Test Coverage Before Refactoring

---

## Phase 3 — Low-Level Design (LLD)

### Design Patterns — Creational
- [ ] Singleton (Thread-Safe)
- [ ] Factory Method
- [ ] Abstract Factory
- [ ] Builder
- [ ] Prototype
- [ ] Object Pool

### Design Patterns — Structural
- [ ] Adapter
- [ ] Bridge
- [ ] Composite
- [ ] Decorator
- [ ] Facade
- [ ] Flyweight
- [ ] Proxy (Virtual, Remote, Protection)

### Design Patterns — Behavioral
- [ ] Observer / Event Bus
- [ ] Strategy
- [ ] Command
- [ ] Chain of Responsibility
- [ ] Template Method
- [ ] State Machine
- [ ] Iterator
- [ ] Visitor
- [ ] Mediator

### Architecture Patterns
- [ ] Clean Architecture (Domain → Use Case → Adapter layers)
- [ ] Hexagonal Architecture (Ports & Adapters)
- [ ] Dependency Rule
- [ ] CQRS Basics
    - [ ] Command vs Query Separation
    - [ ] Read Model vs Write Model
    - [ ] Event Sourcing Intro
    - [ ] When CQRS is Overkill
- [ ] Event-Driven Design

### Domain Modeling
- [ ] Entities vs Value Objects
- [ ] Aggregates & Aggregate Root
- [ ] Repository Pattern
- [ ] Domain Events
- [ ] Bounded Context Intro
- [ ] Ubiquitous Language
- [ ] Anemic vs Rich Domain Model

### Thread Safety in Design
- [ ] Immutability
- [ ] Synchronized Collections
- [ ] ReadWriteLock
- [ ] Concurrent Data Structures
- [ ] Atomic Variables
- [ ] Volatile Keyword

### Extensibility & Testability
- [ ] Open/Closed Applied to Design
- [ ] Feature Flags at Design Level
- [ ] Testable Design Patterns
- [ ] Mock-Friendly Interfaces
- [ ] Arrange-Act-Assert
- [ ] Test Doubles Taxonomy (Mock, Stub, Spy, Fake, Dummy)

---

## Phase 4 — Databases

### SQL & Relational Databases
- [ ] EXPLAIN / Query Plans
- [ ] Index Types (B-tree, Hash, GIN, GiST, Covering Index)
- [ ] Query Optimization
- [ ] JOIN Types and Costs
- [ ] Window Functions
- [ ] Partitioning (Range, List, Hash)
- [ ] Connection Pooling (PgBouncer)
- [ ] Normalization (1NF–3NF) vs Denormalization

### Transactions & ACID
- [ ] Atomicity, Consistency, Isolation, Durability
- [ ] Isolation Levels (READ UNCOMMITTED → SERIALIZABLE)
- [ ] Dirty Read, Non-Repeatable Read, Phantom Read
- [ ] Optimistic vs Pessimistic Locking
- [ ] MVCC (Multi-Version Concurrency Control)
- [ ] Savepoints
- [ ] Deadlock Detection & Prevention

### NoSQL Databases
- [ ] Document (MongoDB) — Data Modeling, Aggregation Pipeline
- [ ] Key-Value (Redis, DynamoDB)
- [ ] Wide-Column (Cassandra, HBase) — Partition Key Design
- [ ] Graph (Neo4j) — Traversal Patterns
- [ ] Time-Series (InfluxDB, TimescaleDB)
- [ ] Choosing NoSQL vs SQL

### CAP Theorem & Consistency Models
- [ ] CAP Theorem — CP vs AP Systems
- [ ] PACELC Extension
- [ ] Eventual Consistency
- [ ] Strong Consistency
- [ ] Causal Consistency
- [ ] Read-Your-Writes
- [ ] Monotonic Reads
- [ ] Linearizability

### Replication
- [ ] Leader-Follower Replication
- [ ] Multi-Leader Replication
- [ ] Leaderless (Dynamo-Style) Replication
- [ ] Replication Lag
- [ ] Read Replicas
- [ ] Failover Strategies
- [ ] Semi-Synchronous Replication
- [ ] Binlog / WAL Streaming

### Sharding & Partitioning
- [ ] Horizontal vs Vertical Partitioning
- [ ] Hash Sharding
- [ ] Range Sharding
- [ ] Directory-Based Sharding
- [ ] Consistent Hashing
- [ ] Shard Key Selection
- [ ] Cross-Shard Queries
- [ ] Resharding Complexity

### Distributed Databases
- [ ] Google Spanner (TrueTime, External Consistency)
- [ ] CockroachDB / YugabyteDB
- [ ] Cassandra (Gossip, Consistent Hashing, Quorum)
- [ ] DynamoDB (Leaderless, Vector Clocks)
- [ ] Vitess for MySQL Sharding
- [ ] NewSQL Tradeoffs

### Search Systems
- [ ] Inverted Index
- [ ] TF-IDF and BM25
- [ ] Elasticsearch Data Model
- [ ] Relevance Scoring
- [ ] Sharding in Elasticsearch
- [ ] Full-Text vs Fuzzy Search
- [ ] Autocomplete (Edge N-Grams)

---

## Phase 5 — Backend Engineering

### API Design at Scale
- [ ] Idempotency Keys
- [ ] Cursor-Based Pagination
- [ ] Bulk APIs
- [ ] PATCH Semantics (Partial Updates)
- [ ] API Versioning Strategies
- [ ] Backward Compatibility
- [ ] Contract Testing
- [ ] OpenAPI Spec-First Design

### Authentication & Authorization
- [ ] Session-Based Auth
- [ ] JWT (Structure, Signing, Expiry, Refresh Tokens)
- [ ] OAuth 2.0 Flows (Authorization Code, PKCE, Client Credentials)
- [ ] OIDC
- [ ] RBAC vs ABAC
- [ ] API Keys Management
- [ ] Token Revocation Strategies
- [ ] SSO and SAML Basics

### Caching Strategies
- [ ] Cache-Aside (Lazy Loading)
- [ ] Write-Through
- [ ] Write-Behind
- [ ] Read-Through
- [ ] Cache Stampede / Thundering Herd
- [ ] Cache Invalidation Strategies
- [ ] TTL Design
- [ ] Distributed Cache (Redis Cluster)
- [ ] Local Cache vs Distributed Cache

### Background Jobs & Queues
- [ ] Job Queue vs Message Queue
- [ ] Task Queues (Celery, Sidekiq, BullMQ)
- [ ] Idempotent Job Design
- [ ] Retry with Exponential Backoff
- [ ] Dead Letter Queues (DLQ)
- [ ] Job Deduplication
- [ ] Cron-Based vs Event-Driven Jobs
- [ ] Worker Concurrency

### Observability
- [ ] Structured Logging (JSON Logs)
- [ ] Log Levels and Sampling
- [ ] Distributed Tracing (OpenTelemetry, Jaeger)
- [ ] Trace Context Propagation
- [ ] Metrics — Counters, Gauges, Histograms, Summaries
- [ ] Prometheus + Grafana
- [ ] Alerting Design
- [ ] SLI / SLO / SLA Definitions
- [ ] Error Budgets

### Connection Pooling & Resource Management
- [ ] DB Connection Pool Sizing (Little's Law)
- [ ] HTTP Keep-Alive and Connection Reuse
- [ ] gRPC Connection Management
- [ ] File Descriptor Limits
- [ ] Graceful Shutdown
- [ ] Health Checks (Liveness vs Readiness vs Startup)

### Testing at Scale
- [ ] Unit, Integration, Contract, Load, Chaos Testing
- [ ] Test Pyramid
- [ ] Testcontainers for Integration Tests
- [ ] Consumer-Driven Contract Tests (Pact)
- [ ] Load Testing (k6, Locust)
- [ ] Chaos Engineering Basics

---

## Phase 6 — Distributed Systems

### Message Queues & Event Streaming
- [ ] Point-to-Point vs Pub/Sub
- [ ] Kafka — Topics, Partitions, Consumer Groups, Offsets
- [ ] Kafka — Producer Acknowledgments (acks=0, 1, all)
- [ ] Kafka — Log Compaction, Retention
- [ ] RabbitMQ — Exchanges (Direct, Fanout, Topic, Headers)
- [ ] Dead Letter Exchanges
- [ ] Message Ordering Guarantees
- [ ] Exactly-Once Semantics
- [ ] Backpressure

### Distributed Caching
- [ ] Redis Data Structures (String, Hash, List, Set, Sorted Set, Stream)
- [ ] Redis Persistence (RDB vs AOF)
- [ ] Redis Cluster — Hash Slots
- [ ] Redis Sentinel
- [ ] Hot Key Problem
- [ ] Cache Warmup Strategies
- [ ] Consistent Hashing for Cache Sharding

### Consistency & Consensus
- [ ] Two-Phase Commit (2PC) and Problems
- [ ] Three-Phase Commit
- [ ] Paxos Overview
- [ ] Raft Consensus (Leader Election, Log Replication)
- [ ] ZooKeeper (ZAB Protocol)
- [ ] etcd Usage Patterns
- [ ] Quorum Reads/Writes
- [ ] Vector Clocks

### Distributed Locks
- [ ] Redis SETNX + TTL
- [ ] Redlock Algorithm
- [ ] ZooKeeper Ephemeral Nodes for Locks
- [ ] Database-Based Locks (SELECT FOR UPDATE)
- [ ] Fencing Tokens
- [ ] Lock Lease Renewal

### Failure Handling Patterns
- [ ] Retry with Exponential Backoff + Jitter
- [ ] Circuit Breaker (Closed, Open, Half-Open)
- [ ] Bulkhead Pattern
- [ ] Timeout Design
- [ ] Fallback Strategies
- [ ] Graceful Degradation
- [ ] Chaos Engineering (Chaos Monkey)

### Idempotency & Exactly-Once Processing
- [ ] Idempotency Keys in APIs
- [ ] At-Least-Once vs At-Most-Once vs Exactly-Once
- [ ] Deduplication Strategies
- [ ] Transactional Outbox Pattern
- [ ] Saga Pattern
    - [ ] Choreography-Based Saga
    - [ ] Orchestration-Based Saga
    - [ ] 2PC vs Saga Tradeoffs

### Service Discovery & Coordination
- [ ] Client-Side vs Server-Side Service Discovery
- [ ] Consul, Eureka, etcd
- [ ] Health-Based Routing
- [ ] DNS-Based Service Discovery
- [ ] Service Mesh Intro (Istio, Linkerd)
- [ ] Sidecar Proxy Pattern

---

## Phase 7 — High-Level Design (HLD)

### Requirement Gathering & Estimation
- [ ] Functional vs Non-Functional Requirements
- [ ] QPS, Storage, Bandwidth Estimation
- [ ] Read-Heavy vs Write-Heavy Classification
- [ ] SLA and SLO Definition
- [ ] Back-of-Envelope Calculations
- [ ] DAU → QPS Conversion
- [ ] Storage Tier Planning

### Load Balancing
- [ ] L4 vs L7 Load Balancing
- [ ] Algorithms (Round Robin, Least Connections, IP Hash, Weighted)
- [ ] Sticky Sessions and Problems
- [ ] Health Checks
- [ ] Global Server Load Balancing (GSLB)
- [ ] AWS ALB / NLB
- [ ] HAProxy vs NGINX

### API Gateway & Reverse Proxy
- [ ] API Gateway Responsibilities (Auth, Rate Limiting, Routing, Logging)
- [ ] NGINX vs Kong vs AWS API Gateway
- [ ] Request Transformation
- [ ] Circuit Breaking at Gateway Level
- [ ] GraphQL Federation Gateway
- [ ] BFF (Backend for Frontend) Pattern

### CDN & Caching Layers
- [ ] CDN — Edge Caching, Origin Shield, Cache Invalidation
- [ ] Cache Hierarchy (Browser → CDN → API Cache → DB Cache)
- [ ] Cache-Control Headers
- [ ] Vary Header
- [ ] Signed URLs
- [ ] CDN for Dynamic vs Static Content

### Scalability Patterns
- [ ] Horizontal vs Vertical Scaling
- [ ] Stateless Service Design
- [ ] Read Replicas for Read Scaling
- [ ] CQRS for Read/Write Scale Separation
- [ ] Database Federation
- [ ] Async Processing for Throughput
- [ ] Event-Driven Decoupling
- [ ] Fan-Out Patterns

### Microservices
- [ ] Microservices vs Monolith — When to Decompose
- [ ] Domain-Driven Decomposition
- [ ] Inter-Service Communication (Sync vs Async)
- [ ] Service Contracts and Versioning
- [ ] Data Ownership (Each Service Owns Its DB)
- [ ] Saga for Distributed Transactions
- [ ] Strangler Fig Migration Pattern
- [ ] API Gateway in Microservices

### Rate Limiting
- [ ] Token Bucket Algorithm
- [ ] Leaky Bucket Algorithm
- [ ] Fixed Window Counter
- [ ] Sliding Window Log
- [ ] Sliding Window Counter (Hybrid)
- [ ] Rate Limiting at API Gateway vs Service Layer
- [ ] Distributed Rate Limiting with Redis
- [ ] Rate Limiting by User, IP, API Key

### Monitoring, Logging & Tracing
- [ ] Metrics Pipeline (Collection → Aggregation → Alerting)
- [ ] Log Aggregation (ELK Stack, Loki)
- [ ] Distributed Tracing — Sampling Strategies
- [ ] SLI-Driven Dashboards
- [ ] Symptom-Based vs Cause-Based Alerting
- [ ] Runbooks
- [ ] On-Call Practices
- [ ] Incident Management

### Reliability Engineering
- [ ] High Availability Patterns
- [ ] Fault Tolerance vs Resilience
- [ ] Multi-AZ Deployment
- [ ] Active-Active vs Active-Passive
- [ ] Data Replication for DR
- [ ] RTO and RPO
- [ ] Graceful Degradation Features

### Multi-Region Systems
- [ ] Data Sovereignty and Compliance
- [ ] Multi-Region Replication Strategies
- [ ] Conflict Resolution (CRDT, Last-Write-Wins)
- [ ] Latency-Based Routing
- [ ] Global Databases (Spanner, CockroachDB)
- [ ] Traffic Shifting and Canary Across Regions
- [ ] Cost vs Latency Tradeoffs

---

## Phase 8 — Security Engineering

### JWT & Session Security
- [ ] JWT Structure (Header.Payload.Signature)
- [ ] HS256 vs RS256 vs ES256
- [ ] Refresh Token Rotation
- [ ] Token Revocation (Blocklist, Short TTL)
- [ ] JWK Endpoint for Key Distribution
- [ ] Session Fixation, CSRF with Cookies
- [ ] HttpOnly + Secure + SameSite Flags

### OAuth 2.0 & OIDC
- [ ] Authorization Code + PKCE Flow
- [ ] Client Credentials Flow (M2M)
- [ ] Implicit Flow (Deprecated — know why)
- [ ] Scopes and Consent
- [ ] ID Token vs Access Token
- [ ] Token Introspection
- [ ] Auth Servers (Keycloak, Auth0, Okta)

### API Security
- [ ] HTTPS Everywhere + HSTS
- [ ] Input Validation and Sanitization
- [ ] SQL Injection Prevention
- [ ] XSS Prevention (CSP Headers)
- [ ] CSRF Tokens
- [ ] Rate Limiting for Security
- [ ] Secret Management (Vault, AWS Secrets Manager)
- [ ] API Key Rotation

### Encryption
- [ ] Symmetric Encryption (AES-256-GCM)
- [ ] Asymmetric Encryption (RSA, ECDH)
- [ ] TLS Certificate Management
- [ ] Key Rotation
- [ ] Encryption at Rest vs In Transit
- [ ] Envelope Encryption (AWS KMS)
- [ ] Hashing (bcrypt for Passwords, SHA-256 for Integrity)

### OWASP Top 10
- [ ] Injection Flaws
- [ ] Broken Authentication
- [ ] Sensitive Data Exposure
- [ ] Broken Access Control
- [ ] Security Misconfiguration
- [ ] XSS
- [ ] Insecure Deserialization
- [ ] Insufficient Logging & Monitoring

---

## Phase 9 — Cloud & Infrastructure

### Docker
- [ ] Container vs VM
- [ ] Dockerfile Best Practices (Multi-Stage, Layer Caching)
- [ ] Image Security (Non-Root User, Minimal Base)
- [ ] Docker Networking (Bridge, Host, Overlay)
- [ ] Docker Compose for Local Dev
- [ ] Container Resource Limits
- [ ] Image Registry (ECR, Docker Hub)

### Kubernetes Basics
- [ ] Pod, Deployment, Service, Ingress, ConfigMap, Secret
- [ ] Rolling Update vs Blue-Green vs Canary
- [ ] HorizontalPodAutoscaler (HPA)
- [ ] Resource Requests vs Limits
- [ ] Liveness vs Readiness vs Startup Probes
- [ ] Namespaces
- [ ] RBAC in Kubernetes
- [ ] PersistentVolumeClaim

### CI/CD
- [ ] GitHub Actions / GitLab CI / Jenkins Pipelines
- [ ] Build → Test → Lint → Containerize → Deploy Pipeline
- [ ] GitOps (ArgoCD, Flux)
- [ ] Deployment Strategies (Blue-Green, Canary, Feature Flags)
- [ ] Rollback Strategy
- [ ] Environment Promotion (dev → staging → prod)
- [ ] Secret Injection in Pipelines

### Cloud Fundamentals (AWS / GCP / Azure)
- [ ] Compute (EC2/GKE/AKS, Lambda/Cloud Functions)
- [ ] Storage (S3/GCS/Blob, RDS, DynamoDB/Firestore)
- [ ] Networking (VPC, Subnets, Security Groups, ALB/NLB)
- [ ] IAM (Roles, Policies, Least Privilege)
- [ ] Managed Services vs Self-Hosted Tradeoffs
- [ ] Cost Optimization Basics
- [ ] Multi-AZ and Multi-Region Setup

---

## Phase 10 — Scalability & Performance Engineering

### Performance Profiling
- [ ] CPU Profiling — Flame Graphs
- [ ] Memory Profiling — Heap Dumps, GC Pressure
- [ ] I/O Profiling — Disk and Network Bottlenecks
- [ ] Profiling Tools (pprof, async-profiler, py-spy)
- [ ] Database Query Profiling
- [ ] Load Testing Methodology (k6, Gatling)

### Database Performance
- [ ] Query Plan Analysis
- [ ] Index Strategies for Complex Queries
- [ ] Connection Pool Tuning
- [ ] Read Replica Routing
- [ ] Denormalization for Read Paths
- [ ] Materialized Views
- [ ] Batching and Bulk Operations
- [ ] N+1 Detection and Fix (Dataloader Pattern)

### Caching & Throughput
- [ ] Cache Hit Ratio Optimization
- [ ] Bloom Filters for Negative Caching
- [ ] Request Coalescing
- [ ] Batching API Calls
- [ ] Async Pre-Computation
- [ ] Write Amplification vs Read Amplification Tradeoffs

### Concurrency & Throughput
- [ ] Thread Pool Sizing (Little's Law)
- [ ] Event Loop Model (Node.js, Nginx)
- [ ] Non-Blocking I/O Patterns
- [ ] Async Processing Chains
- [ ] Backpressure Handling
- [ ] Lock Contention Profiling

---

## Phase 11 — Reliability & Production Engineering

### SLI / SLO / SLA
- [ ] Defining Meaningful SLIs (Latency p99, Availability, Error Rate)
- [ ] SLO Target Setting
- [ ] Error Budget Calculation
- [ ] Error Budget Burn Rate Alerts
- [ ] SLA as Business Contract
- [ ] Toil Measurement
- [ ] Rolling vs Calendar SLO Windows

### Incident Management
- [ ] On-Call Rotation Design
- [ ] Incident Severity Levels
- [ ] Incident Commander Role
- [ ] Runbooks and Playbooks
- [ ] Blameless Post-Mortems
- [ ] 5 Whys Analysis
- [ ] Action Items Tracking
- [ ] GameDay Exercises

### Disaster Recovery
- [ ] RTO (Recovery Time Objective)
- [ ] RPO (Recovery Point Objective)
- [ ] Backup Strategies (Full, Incremental, Differential)
- [ ] Point-in-Time Recovery
- [ ] Multi-Region Failover
- [ ] DB Replication for DR
- [ ] DR Testing Frequency
- [ ] Failover Runbook

### Chaos Engineering
- [ ] Principles of Chaos Engineering
- [ ] Steady-State Hypothesis
- [ ] Chaos Monkey, Chaos Toolkit
- [ ] Network Partition Injection
- [ ] Latency Injection
- [ ] Resource Exhaustion Tests
- [ ] Chaos in Staging Before Production

---

## Phase 12 — HLD Case Studies

> For each: identify prerequisites → understand what it teaches → design end-to-end

- [ ] URL Shortener
    - [ ] Hashing & Encoding (Base-62)
    - [ ] Write Path — ID Generation
    - [ ] Read Path — Redirect at Scale
    - [ ] Analytics Fan-Out
    - [ ] Hot Key Problem in Cache

- [ ] Chat System (WhatsApp)
    - [ ] Real-Time Messaging (WebSocket)
    - [ ] Message Ordering & Storage
    - [ ] Online Presence
    - [ ] Multi-Device Sync
    - [ ] Message Delivery Guarantees (ACK, At-Least-Once)

- [ ] Instagram / Photo Sharing
    - [ ] Blob Storage Pipeline
    - [ ] Feed Generation (Push vs Pull vs Hybrid)
    - [ ] Notification Fan-Out
    - [ ] Image Processing Pipeline
    - [ ] Geo-Distributed Reads

- [ ] YouTube / Video Platform
    - [ ] Video Upload Pipeline
    - [ ] Transcoding Workers (Adaptive Bitrate — HLS/DASH)
    - [ ] CDN for Video Delivery
    - [ ] View Count Accuracy
    - [ ] Recommendation System Data Flow

- [ ] Netflix
    - [ ] Open Connect CDN
    - [ ] Chaos Engineering in Production
    - [ ] A/B Testing Infrastructure
    - [ ] Personalization Pipeline
    - [ ] Multi-Region Active-Active

- [ ] Uber / Cab Booking
    - [ ] Driver Location Ingestion (Geohashing / Quadtree)
    - [ ] Supply-Demand Matching
    - [ ] Surge Pricing Data Flow
    - [ ] Trip State Machine
    - [ ] GPS Stream Processing

- [ ] Twitter / X
    - [ ] Tweet Fanout (Push vs Pull by Follower Count)
    - [ ] Trending Topic Computation
    - [ ] Social Graph Traversal
    - [ ] Timeline Generation
    - [ ] Search Indexing at Write Time

- [ ] Google Docs (Collaborative Editing)
    - [ ] Operational Transformation (OT)
    - [ ] CRDT for Conflict Resolution
    - [ ] Cursor Presence (WebSocket)
    - [ ] Version History
    - [ ] Permission Model

- [ ] Dropbox / File Storage
    - [ ] File Chunking and Deduplication
    - [ ] Delta Sync
    - [ ] Conflict Resolution
    - [ ] Offline Support
    - [ ] Resumable Uploads

- [ ] Payment Gateway
    - [ ] Idempotency Keys
    - [ ] Double-Spend Prevention
    - [ ] Double-Entry Ledger Design
    - [ ] PCI DSS Constraints
    - [ ] Reconciliation
    - [ ] Saga for Multi-Step Payments

- [ ] Notification System
    - [ ] Multi-Channel Delivery (Push, SMS, Email)
    - [ ] Priority Queues
    - [ ] User Preference Service
    - [ ] Delivery Tracking
    - [ ] Retry Logic and DLQ

- [ ] Food Delivery (DoorDash / Zomato)
    - [ ] Restaurant Catalog Search
    - [ ] Order Routing
    - [ ] Delivery Assignment
    - [ ] Real-Time ETA
    - [ ] Analytics Pipeline

- [ ] E-Commerce Platform
    - [ ] Product Catalog Search
    - [ ] Inventory Management (Distributed Locks for Stock)
    - [ ] Flash Sale Design (Oversell Prevention)
    - [ ] Cart Service
    - [ ] Order State Machine

- [ ] Ticket Booking (BookMyShow)
    - [ ] Seat Reservation Under Contention
    - [ ] Optimistic vs Pessimistic Locking for Seats
    - [ ] Temporary Hold + Payment Confirmation
    - [ ] Payment Timeout and Rollback
    - [ ] Waitlist

---

## Phase 13 — LLD Practice Problems

> For each: draw class diagram first → implement key classes → write unit tests

- [ ] Parking Lot
    - [ ] Slot Types & Multi-Floor
    - [ ] Pricing Strategy (Strategy Pattern)
    - [ ] Extensible Vehicle Types
    - [ ] Entry/Exit Flow

- [ ] Elevator System
    - [ ] State Machine per Elevator
    - [ ] Scheduling Algorithm (SCAN / LOOK)
    - [ ] Multi-Elevator Coordination

- [ ] ATM System
    - [ ] Card Authentication
    - [ ] Transaction Atomicity
    - [ ] Cash Dispensing Logic
    - [ ] Error States & Audit Trail

- [ ] Splitwise
    - [ ] Group Expense Management
    - [ ] Debt Simplification (Graph Algorithm)
    - [ ] Transaction History

- [ ] Chess
    - [ ] Board Abstraction
    - [ ] Piece Hierarchy (Polymorphism)
    - [ ] Move Validation
    - [ ] Check / Checkmate Detection
    - [ ] Undo (Command Pattern)

- [ ] Cricbuzz Score Engine
    - [ ] Match Lifecycle State Machine
    - [ ] Scoring Engine
    - [ ] Player Stats
    - [ ] Real-Time Commentary

- [ ] Library Management System
    - [ ] Catalog & Search
    - [ ] Borrowing & Reservations
    - [ ] Late Fee Calculation
    - [ ] Notification on Availability

- [ ] Food Delivery LLD
    - [ ] Restaurant & Menu Model
    - [ ] Order Lifecycle State Machine
    - [ ] Delivery Assignment

- [ ] Cab Booking LLD
    - [ ] Driver & Rider State Machine
    - [ ] Trip Lifecycle
    - [ ] Pricing Strategy

- [ ] LRU Cache
    - [ ] O(1) Get/Put (HashMap + Doubly Linked List)
    - [ ] Thread-Safe Implementation
    - [ ] Eviction Policy
    - [ ] Serialization

- [ ] Logging Framework
    - [ ] Log Levels
    - [ ] Appenders (Console, File, Remote)
    - [ ] Formatters
    - [ ] Async Buffer
    - [ ] Log Rotation

- [ ] Rate Limiter
    - [ ] Token Bucket Implementation
    - [ ] Sliding Window Implementation
    - [ ] Per-User Configuration
    - [ ] Distributed Extension (Redis)

- [ ] Job Scheduler
    - [ ] Cron + One-Time Jobs
    - [ ] Priority Queue
    - [ ] Retry & Cancellation
    - [ ] Execution Tracking

---

## Phase 14 — Interview Preparation

### HLD Interview Framework
- [ ] Step 1 — Requirements (Functional + NFR, 5 min)
- [ ] Step 2 — Capacity Estimation (5 min)
- [ ] Step 3 — High-Level Design (10 min)
- [ ] Step 4 — Component Deep Dive (15 min)
- [ ] Step 5 — Bottlenecks + Tradeoffs (5 min)
- [ ] Driving the Conversation (Not Being Led)
- [ ] Handling Unknown Areas Gracefully
- [ ] Asking Clarifying Questions at Right Time

### LLD Interview Framework
- [ ] Step 1 — Clarify Requirements and Constraints (5 min)
- [ ] Step 2 — Identify Core Entities and Relationships (5 min)
- [ ] Step 3 — Class Diagram (10 min)
- [ ] Step 4 — Code Key Classes and Methods (20 min)
- [ ] Step 5 — Extensibility + Edge Cases (5 min)
- [ ] Proactively Address Thread Safety
- [ ] Pattern Selection Without Being Asked

### Key Tradeoffs to Master
- [ ] SQL vs NoSQL
- [ ] Sync vs Async Communication
- [ ] Push vs Pull Feed Generation
- [ ] Monolith vs Microservices
- [ ] Consistency vs Availability
- [ ] Strong vs Eventual Consistency
- [ ] Build vs Buy for Infrastructure

### Mock Interviews
- [ ] 10 HLD Mock Interviews (solo or partner)
- [ ] 10 LLD Mock Interviews (solo or partner)
- [ ] Self-Review Recordings for Each Mock
- [ ] Timed Practice (45–60 min per session)
- [ ] Company-Specific Problem Research

---

## Phase 15 — Real-World Projects

### Foundation Projects
- [ ] Mini HTTP Server from scratch (Go or Python)
- [ ] TCP Chat Server (multi-client)
- [ ] DNS Resolver Trace CLI Tool
- [ ] Concurrency Lab (producer-consumer, dining philosophers, bounded buffer)

### LLD Projects
- [ ] OOP Design Lab — Refactor a bad codebase to SOLID
- [ ] Manual DI Container
- [ ] Thread-Safe LRU Cache with unit tests
- [ ] Rate Limiter (all algorithms, Redis-backed distributed)
- [ ] Job Scheduler with cron + priority + retry

### Database Projects
- [ ] Key-Value Store with WAL and Compaction (LSM-tree concept)
- [ ] Consistent Hashing Ring with Virtual Nodes
- [ ] Inverted Index from Scratch with TF-IDF
- [ ] Query Optimization Lab (10 slow queries fixed with EXPLAIN ANALYZE)

### Backend Projects
- [ ] Auth Service (JWT RS256 + Refresh Token Rotation + RBAC)
- [ ] Async Job System (Transactional Outbox + DLQ + Retry)
- [ ] Observability Stack (OpenTelemetry + Jaeger + Prometheus + Grafana)

### Distributed Systems Projects
- [ ] Event-Driven Order Pipeline (Kafka + Saga + Idempotent Consumers)
- [ ] Distributed Lock Service (Redis + Fencing Tokens + Lease Renewal)
- [ ] Circuit Breaker Library (State Machine + Metrics)

### HLD Projects
- [ ] URL Shortener — Full Implementation + Load Test at 10k RPS
- [ ] Notification System — Multi-Channel + Priority + Delivery Tracking
- [ ] Rate Limiter Service — Distributed, Multi-Algorithm

### Infrastructure Projects
- [ ] Containerize Multi-Service App (Docker + Multi-Stage Build)
- [ ] K8s Deployment with HPA, Probes, Rolling Update, Rollback
- [ ] Full CI/CD Pipeline (GitHub Actions → Docker → K8s + Canary)
- [ ] AWS 3-Tier App (VPC + ALB + ECS/EKS + RDS Multi-AZ + Elasticache)

---

## Revision Checklist

### Core Concepts Revision
- [ ] OSI Model + TCP/IP Stack
- [ ] ACID vs BASE
- [ ] CAP Theorem + PACELC
- [ ] Consistency Models (Strong → Eventual)
- [ ] Consensus Algorithms (Raft, Paxos overview)
- [ ] Consistent Hashing
- [ ] Distributed Locking + Fencing Tokens
- [ ] Saga Pattern (Choreography vs Orchestration)
- [ ] Idempotency Patterns
- [ ] Circuit Breaker State Machine
- [ ] All Rate Limiting Algorithms
- [ ] All Caching Strategies
- [ ] Feed Generation Strategies (Push vs Pull vs Hybrid)
- [ ] Replication Strategies (Leader-Follower, Multi-Leader, Leaderless)
- [ ] Sharding Strategies + Consistent Hashing
- [ ] OAuth 2.0 Flows
- [ ] JWT Security Best Practices

### Design Pattern Revision
- [ ] Creational Patterns — All 6
- [ ] Structural Patterns — All 7
- [ ] Behavioral Patterns — All 9
- [ ] SOLID Violations — Spot and Fix
- [ ] Hexagonal Architecture
- [ ] CQRS + Event Sourcing

### HLD Component Revision
- [ ] Load Balancing Algorithms
- [ ] API Gateway Responsibilities
- [ ] CDN Invalidation Strategies
- [ ] Multi-Region Failure Scenarios
- [ ] SLO + Error Budget Math
- [ ] RTO / RPO Definitions and Tradeoffs

---

*Last updated: —*
*Status: In Progress*
