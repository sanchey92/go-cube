# Go-Cube

A lightweight container orchestration system built with Go, inspired by Kubernetes architecture. Go-Cube provides distributed task scheduling, health monitoring, and automated container lifecycle management using Docker.

## Features

- **Distributed Architecture**: Manager-Worker pattern for scalable task distribution
- **Docker Integration**: Native Docker API integration for container management
- **Intelligent Scheduling**: Round-robin scheduler with extensible scheduling algorithms
- **Health Monitoring**: Automated health checks and task restart policies
- **Graceful Shutdown**: Proper signal handling and concurrent goroutine management
- **RESTful API**: HTTP endpoints for task management and monitoring
- **Concurrent Task Processing**: Efficient goroutine-based task execution
- **In-Memory Storage**: Fast task and event storage with thread-safe operations

## Getting Started

### Prerequisites

- Go 1.25.3 or higher
- Docker Engine installed and running
- Docker daemon accessible via socket

### Installation

```bash
git clone https://github.com/sanchey92/go-cube.git
cd go-cube
go mod download
```

### Running the Application

#### Mode 1: Single Worker

Start a worker node:

```bash
go run main.go -mode worker -host localhost -port 8081
```

#### Mode 2: Manager with Multiple Workers

Start worker nodes:

```bash
# Terminal 1 - Worker 1
go run main.go -mode worker -host localhost -port 8081 -name worker-1

# Terminal 2 - Worker 2
go run main.go -mode worker -host localhost -port 8082 -name worker-2
```

Start manager:

```bash
# Terminal 3 - Manager
go run main.go -mode manager -host localhost -port 8080 \
  -workers "localhost:8081,localhost:8082"
```

#### Mode 3: All-in-One (Development)

Start both manager and worker on the same machine:

```bash
go run main.go -mode all -host localhost -port 8081 -manager-port 8080
```

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-mode` | Application mode: `worker`, `manager`, or `all` | `worker` |
| `-host` | Host address to bind | `localhost` |
| `-port` | Port number for worker or manager | `8081` |
| `-manager-port` | Manager port (used in 'all' mode) | `8080` |
| `-workers` | Comma-separated worker addresses | `""` |
| `-name` | Worker name (optional) | auto-generated |

## API Reference

### Manager API

#### Create Task
```bash
POST http://localhost:8080/tasks
Content-Type: application/json

{
  "ID": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "State": 2,
  "Timestamp": "2025-11-23T10:00:00Z",
  "Task": {
    "ID": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "Name": "my-task",
    "Image": "nginx:latest",
    "CPU": 0.5,
    "Memory": 536870912,
    "Disk": 1073741824,
    "Env": ["ENV=production"],
    "ExposedPorts": {
      "80/tcp": {}
    },
    "RestartPolicy": "always",
    "HealthCheck": "/health"
  }
}
```

#### Get All Tasks
```bash
GET http://localhost:8080/tasks
```

#### Stop Task
```bash
DELETE http://localhost:8080/tasks/{taskID}
```

### Worker API

#### Get Worker Tasks
```bash
GET http://localhost:8081/tasks
```

#### Get Worker Statistics
```bash
GET http://localhost:8081/stats
```

#### Stop Task on Worker
```bash
DELETE http://localhost:8081/tasks/{taskID}
```

## Project Structure

```
go-cube/
├── main.go                          # Application entry point
├── internal/
│   ├── client/
│   │   ├── container/
│   │   │   └── docker.go           # Docker API client
│   │   └── http/
│   │       └── client.go           # HTTP client wrapper
│   ├── http/
│   │   ├── manager/
│   │   │   ├── server.go           # Manager HTTP server
│   │   │   ├── router.go           # Manager routes
│   │   │   └── handlers.go         # Manager HTTP handlers
│   │   └── worker/
│   │       ├── server.go           # Worker HTTP server
│   │       ├── router.go           # Worker routes
│   │       └── handlers.go         # Worker HTTP handlers
│   ├── models/
│   │   └── container.go            # Container-related models
│   ├── services/
│   │   ├── manager/
│   │   │   └── manager.go          # Manager service logic
│   │   ├── worker/
│   │   │   └── worker.go           # Worker service logic
│   │   ├── scheduler/
│   │   │   └── scheduler.go        # Scheduling algorithms
│   │   ├── task/
│   │   │   ├── task.go             # Task models
│   │   │   └── state.go            # Task state management
│   │   ├── node/
│   │   │   └── node.go             # Node representation
│   │   └── stats/
│   │       └── stats.go            # System statistics
│   └── store/
│       └── store.go                # In-memory data store
├── pkg/
│   ├── errors/
│   │   └── errors.go               # Error definitions
│   └── utils/
│       └── utils.go                # Utility functions
└── go.mod
```

## Concurrency Model

### Manager Goroutines
- **ProcessTasks**: Dequeues and assigns tasks to workers (every 10s)
- **UpdateTasks**: Syncs task states from workers (every 15s)
- **DoHealthChecks**: Monitors task health and triggers restarts (every 60s)
- **HTTP Server**: Handles incoming API requests

### Worker Goroutines
- **RunTasks**: Processes tasks from queue (every 10s)
- **UpdateTasks**: Updates task states from containers (every 15s)
- **CollectStats**: Gathers system metrics (every 15s)
- **HTTP Server**: Handles incoming API requests

All goroutines are coordinated using:
- `context.Context` for cancellation propagation
- `sync.WaitGroup` for graceful shutdown
- `sync.RWMutex` for thread-safe data access

## Task States

Tasks transition through the following states:

1. **Pending** (0): Task created but not scheduled
2. **Scheduled** (1): Task assigned to a worker
3. **Running** (2): Container is running
4. **Completed** (3): Task finished successfully
5. **Failed** (4): Task failed or container exited with error

## Health Checks

The Manager performs health checks on running tasks:
- Configurable health check endpoint per task
- Automatic restart on health check failures
- Maximum restart count limit (default: 3)
- Exponential backoff between restarts

## Example Usage

### Running an Nginx Task

1. Start the infrastructure:
```bash
# Terminal 1: Worker
go run main.go -mode worker -port 8081

# Terminal 2: Manager
go run main.go -mode manager -port 8080 -workers "localhost:8081"
```

2. Submit a task:
```bash
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "ID": "123e4567-e89b-12d3-a456-426614174000",
    "State": 2,
    "Task": {
      "ID": "123e4567-e89b-12d3-a456-426614174000",
      "Name": "nginx-server",
      "Image": "nginx:latest",
      "CPU": 0.5,
      "Memory": 268435456,
      "ExposedPorts": {
        "80/tcp": {}
      },
      "RestartPolicy": "always",
      "HealthCheck": "/health"
  }'
```

3. Check task status:
```bash
curl http://localhost:8080/tasks
```

4. Stop the task:
```bash
curl -X DELETE http://localhost:8080/tasks/123e4567-e89b-12d3-a456-426614174000
```

## Configuration

### Timeouts
Defined in `internal/services/manager/manager.go`:
- Task update interval: 15s
- Health check interval: 60s
- Process tasks interval: 10s
- HTTP client timeout: 10s

### Server Timeouts
Defined in `main.go`:
- Read timeout: 15s
- Write timeout: 15s
- Idle timeout: 60s
- Shutdown timeout: 30s

## Development

### Building

```bash
# Build binary
go build -o go-cube main.go

# Run binary
./go-cube -mode all -port 8081 -manager-port 8080
```

## Error Handling

The system includes comprehensive error handling:
- Custom error types in `pkg/errors/errors.go`
- HTTP error responses with appropriate status codes
- Graceful degradation on worker failures
- Task re-queuing on network failures

## License

This project is part of the Go Education series and is available for educational purposes.

