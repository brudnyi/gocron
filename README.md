# GoCron Service

GoCron is a flexible and reliable service for scheduling and managing time-based jobs. It allows clients to create, monitor, and manage scheduled tasks that trigger webhooks at specified intervals. This service is designed to be highly available and scalable, making it suitable for a wide range of use cases, from simple reminders to complex, recurring tasks.

## Features

- **Dynamic Job Scheduling:** Schedule jobs to run after a specified delay and to repeat a certain number of times.
- **Webhook Integration:** Trigger any HTTP/HTTPS endpoint with configurable methods, headers, and data.
- **Custom Job IDs:** Assign custom IDs to jobs for easy tracking and management.
- **Job Status Tracking:** Monitor the status of each job, from "ACTIVE" to "COMPLETED".
- **Scalable Architecture:** Built with a robust architecture that can handle a large number of concurrent jobs.

## Getting Started

### Prerequisites

- [Go](https://golang.org/) (version 1.20 or higher)
- [PostgreSQL](https://www.postgresql.org/)

### Installation

1. **Clone the repository:**
   ```sh
   git clone https://gitlab.uis.dev/service/gocron.git
   cd gocron
   ```

2. **Create a configuration file:**
   Create a `config.yaml` file in the root of the project with the following content:
   ```yaml
   server:
     port: 8080

   postgres:
     url: "postgres://user:password@localhost:5432/gocron?sslmode=disable"
   ```

3. **Run database migrations:**
   ```sh
   make migrate-up
   ```

4. **Build and run the service:**
   ```sh
   go build ./cmd/gocron
   ./gocron
   ```

The service will now be running on `http://localhost:8080`.

## API Documentation

### Health Check

- **Endpoint:** `GET /`
- **Description:** Checks the health of the service.
- **Success Response (200 OK):**
  ```json
  {
    "status": "ok"
  }
  ```

### Create a New Job

- **Endpoint:** `POST /jobs`
- **Description:** Creates a new job with the specified parameters.
- **Request Body:**
  ```json
  {
    "custom_id": "my-unique-job-id",
    "delay": 10,
    "repeat": 5,
    "webhook": {
      "url": "https://example.com/webhook",
      "method": "POST",
      "headers": {
        "Content-Type": "application/json"
      },
      "json": {
        "key": "value"
      }
    }
  }
  ```
  - `custom_id` (optional, string): A unique identifier for the job.
  - `delay` (required, integer): The delay in seconds before the first execution.
  - `repeat` (required, integer): The number of times the job should repeat.
  - `webhook` (required, object): The webhook to be triggered.
    - `url` (required, string): The URL of the webhook.
    - `method` (required, string): The HTTP method to be used (e.g., "POST", "GET").
    - `headers` (optional, object): A map of HTTP headers.
    - `json` (optional, object): A JSON object to be sent as the request body.

- **Success Response (201 Created):**
  ```json
  {
    "id": 1,
    "custom_id": "my-unique-job-id",
    "created_at": "2025-08-03T10:00:00Z",
    "updated_at": "2025-08-03T10:00:00Z",
    "delay": 10,
    "repeat": 5,
    "webhook": {
      "url": "https://example.com/webhook",
      "method": "POST",
      "headers": {
        "Content-Type": "application/json"
      },
      "json": {
        "key": "value"
      }
    },
    "status": "ACTIVE",
    "executions": 0,
    "deadline_at": "2025-08-03T10:00:50Z",
    "completed_at": null
  }
  ```