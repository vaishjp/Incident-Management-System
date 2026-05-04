#  Incident Management System (IMS) 

##  1. Introduction
Modern distributed systems operate at massive scale, where failures are inevitable rather than exceptional. Systems composed of APIs, databases, caches, and asynchronous queues continuously generate high volumes of signals such as errors, latency spikes, and resource exhaustion.

This project implements a Mission-Critical Incident Management System (IMS) designed to:
* Ingest high-volume signals reliably
* Aggregate and deduplicate alerts
* Track incident lifecycle using structured workflows
* Enforce Root Cause Analysis (RCA)
* Provide real-time visibility through a dashboard

The system simulates real-world Site Reliability Engineering (SRE) practices used in production environments.

---

##  2. Objectives
The primary goals of this system are:
* Handle high-throughput signal ingestion (10,000/sec)
* Prevent system overload using backpressure mechanisms
* Reduce alert noise via debouncing logic
* Maintain data separation for performance and reliability
* Implement a state-driven incident lifecycle
* Enforce mandatory RCA before closure
* Provide real-time UI dashboard for monitoring

---

##  3. System Overview
This system is designed with real-world SRE principles such as fault isolation, backpressure handling, and observability to simulate production-grade incident management workflows.

### High-Level Flow: 
 Signals ➔ API ➔ In-Memory Queue ➔ Workers ➔ DBs ➔ Dashboard

---

##  4. Architecture

###  Core Components
| Layer                              | Description                              |
| ---------------------------------  | ---------------------------------------- |
| **Ingestion Layer**                | Accepts incoming signals via API         |
| **Processing Layer**               | Handles async processing using workers   |
| **Storage Layer**                  | Stores structured + unstructured data    |
| **Cache Layer**                    | Maintains real-time state                |
| **Workflow Engine**                | Manages incident lifecycle               |
| **UI Dashboard**                   | Displays incidents and allows RCA        |

###  Architecture Diagram (Conceptual)
```text
          +----------------------+
          |   Signal Producers   |
          +----------+-----------+
                     |
                     v
          +----------------------+
          |   Ingestion API      |
          +----------+-----------+
                     |
          (Buffered Channel Queue)
                     |
                     v
          +----------------------+
          |   Worker Pool        |
          +----------+-----------+
                     |
     +---------------+-------------------+
     |                                   |
     v                                   v
+------------+                    +---------------+
| MongoDB    |                    | PostgreSQL    |
| (Raw Logs) |                    | (Work Items)  |
+------------+                    +---------------+
                     |
                     v
               +------------+
               |   Redis    |
               | (Cache)    |
               +------------+
                     |
                     v
               +------------+
               |  Frontend  |
               +------------+

```

## Key Features
* High-throughput signal ingestion with async processing
* Redis-based debouncing to reduce alert noise
* Incident lifecycle management using state machine
* Mandatory RCA enforcement before closure
* MTTR calculation for incident resolution
* Real-time dashboard with raw signal visualization
* Rate limiting and backpressure handling

##  5. Key Components

###  5.1 Ingestion & Processing
*   Built using **Go (Fiber framework)**.
*   Handles high throughput via:
    *   **Buffered channels** (queue).
    *   **Non-blocking** request handling.
*   Returns `202 Accepted` immediately.
*   **Key Feature:** Prevents API slowdown due to database latency.

###  5.2 Backpressure Mechanism
To prevent system crashes:
*   **Rate limiting** at the API level.
*   **Channel capacity limit** (e.g., 100,000 signals).
*   Rejects excess requests with `503 Service Unavailable`.

###  5.3 Debouncing Logic
*   **Problem:** Thousands of identical signals cause alert fatigue.
*   **Solution:**
    *   **Redis-based locking** (`SETNX`).
    *   Groups signals within a 10-second window.
    *   Creates only one work item per incident.

###  5.4 Data Storage Strategy
| Database                     | Purpose                              |
| ---------------------------- | ------------------------------------ |
| **MongoDB**                  | Raw signal storage (audit logs)      |
| **PostgreSQL**               | Incident lifecycle (source of truth) |
| **Redis**                    | Real-time state + debouncing         |

### 5.5 Workflow Engine
Implements the **State Design Pattern**:
*   **States:** `OPEN` ➔ `INVESTIGATING` ➔ `RESOLVED` ➔ `CLOSED`
*   **Rules:**
    *   Cannot close without an RCA (Root Cause Analysis).
    *   State transitions are strictly validated.

### 5.6 Alerting Strategy
Implements the **Strategy Pattern**:
| Severity | Example |
| :--- | :--- |
| **P0 / CRITICAL** | RDBMS failure |
| **P1 / HIGH** | API latency |
| **P2 / MEDIUM** | Cache issues |

###  5.7 Frontend Dashboard
Built using **React**.
*   **Features:**
    *   Live incident feed.
    *   Severity-based sorting.
    *   Incident details view.
    *   RCA submission form.
    *   Raw signal visualization.

---

##  6. Features Implemented

### Functional Features
*   High-throughput ingestion
*   Async processing with workers
*   Incident lifecycle management
*   Mandatory RCA enforcement
*   MTTR calculation
*   Real-time dashboard
*   Raw signal visualization

### Non-Functional Features
*   Rate limiting
*   Backpressure handling
*   Basic fault tolerance via async processing and backpressure
*   Data consistency (PostgreSQL transactions)
*   Scalability via concurrent worker pool
*   Observability (`/health` + logs)

---

##  7. Observability
*   `/health` endpoint exposed.
*   Throughput logs generated every 5 seconds detailing:
    *   Signals/sec
    *   Queue depth
    *   Processed count

---

##  8. MTTR Calculation
**Mean Time To Repair (MTTR):**
```text
MTTR = RCA Submission Time - First Signal Time
*Automatically calculated and stored upon incident closure.*

---
```
##  9. Challenges & Solutions

*   **Challenge 1: High Throughput Handling**
    *   **Problem:** System overload during bursts.
    *   **Solution:** Buffered channels + async workers.
      
*   **Challenge 2: Database Bottleneck**
    *   **Problem:** DB writes slowing down the API.
    *   **Solution:** Decoupled ingestion from persistence.
      
*   **Challenge 3: Alert Fatigue**
    *   **Problem:** Duplicate alerts flooding the system.
    *   **Solution:** Redis-based debouncing.
      
*   **Challenge 4: Data Consistency**
    *   **Problem:** State corruption during transitions.
    *   **Solution:** PostgreSQL ACID transactions.
      
*   **Challenge 5: UI Data Mismatch**
    *   **Problem:** Backend `snake_case` vs frontend `camelCase`.
    *   **Solution:** Correct field mapping.
      
*   **Challenge 6: Docker & Environment Issues**
    *   **Problem:** DB auth + container conflicts.
    *   **Solution:** Clean container reset + proper `.env` configuration.

---
## 🔌 Key API Endpoints

GET /incidents  
→ Fetch all active incidents  

POST /incidents/{id}/acknowledge  
→ Move incident to INVESTIGATING  

POST /rca  
→ Submit RCA and resolve incident  

POST /incidents/{id}/close  
→ Close incident  

GET /incidents/{id}/signals  
→ Fetch raw signals from MongoDB  

GET /health  
→ System health check
---

##  10. Future Enhancements
*   Real-time WebSocket updates for the dashboard.
*   Advanced alerting (Email/SMS/Slack integrations).
*   AI-based anomaly detection.
*   Multi-tenant support.
*   Kubernetes deployment (Helm charts).
*   Role-based access control (RBAC).
*   Dashboard analytics (charts, trends).

---

##  11. Learnings
*   Real-world SRE system design.
*   Handling concurrency in Go.
*   Distributed system patterns.
*   Database selection strategy.
*   Debugging Docker environments.
*   End-to-end system integration.

---

## 12. Tech Stack

| Layer                      | Technology             |
| -------------------------- | ---------------------- |
| **Backend**                | Go (Fiber)             |
| **Frontend**               | React                  |
| **Database**               | PostgreSQL, MongoDB    |
| **Cache**                  | Redis                  |
| **Containerization**       | Docker                 |
| **API**                    | REST                   |

---
## 13. How to Run

### Prerequisites
- Docker & Docker Compose installed
- Go installed (optional for local run)
- Node.js (for frontend)

### Step 1: Start services
```bash
docker-compose up -d
```
### Step 2: Run backend
```bash
cd backend
go run .
```
### Step 3: Run frontend
```bash
cd frontend
npm install
npm run dev
```
### Step 4: Access application

Frontend: http://localhost:5173  
Backend: http://localhost:3000


##  14. Repository Structure
```text
Incident-Management-System/
 ├── backend/
 ├── frontend/
 ├── docker-compose.yml
 ├── PROOF_OF_WORK_IMS
 └── README.md
```
##  Demo / Proof of Work

Screenshots demonstrating:
- Incident dashboard
- RCA workflow
- Raw signal logs

Located in: /PROOF_OF_WORK_IMS


##  15. Conclusion
This project successfully demonstrates the design and implementation of a resilient, scalable Incident Management System. It incorporates real-world SRE practices, distributed system design principles, fault tolerance, and observability. The system is capable of handling high-volume signals while maintaining performance, reliability, and usability.
