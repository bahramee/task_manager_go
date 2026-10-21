# Task Manager --- Business Overview

## 1. Purpose

Task Manager is a small backend service for managing tasks through a
REST API. A client application can create tasks, view them, update their
details, and delete them. Task data is stored in PostgreSQL so it
remains available when the API restarts.

The goal is to build a service that is simple to run and maintain, with
clear responsibilities in the code and enough tests and documentation
for another developer to understand it.

## 2. Scope

### Required features

The service must provide REST endpoints to:

-   Create a task.
-   Retrieve one task by its ID.
-   List tasks.
-   Update an existing task.
-   Delete a task.

The API must use Gin for HTTP handling and PostgreSQL for persistent
storage.

The project must also include:

-   Unit and integration tests, with at least 70% code coverage.
-   Database mocking for tests that should not require a real database.
-   Docker and Docker Compose configuration so the service and
    PostgreSQL can be run locally.
-   A multi-stage Docker build.
-   OpenAPI documentation, available through a readable Swagger
    endpoint.
-   A README with setup instructions, API examples, testing
    instructions, design decisions, and trade-offs.
-   Basic Prometheus metrics for request counts, request latency, and
    task counts, plus simple tracing.

### Optional features

These can be added after the required features work reliably:

-   Redis cache-aside for `GET /tasks`, with cache invalidation after
    task changes.
-   Filtering by task status and assignee.
-   Pagination for task lists.
-   A small load test or benchmark, with relevant `pprof` findings
    documented in the README.

## 3. Task data

The task is the main resource managed by the service. The final fields
should be chosen before implementing the database schema and API.

A reasonable starting point is:

-   `id`: unique identifier.
-   `title`: short task name.
-   `description`: additional details; optional if appropriate.
-   `status`: the task's current state.
-   `assignee`: the person responsible for the task, if assignment is in
    scope.
-   `created_at`: when the task was created.
-   `updated_at`: when the task was last changed.

The assignment does not define all of these fields or their exact rules.
Treat them as proposed design choices, not as requirements supplied by
the interviewer. Keep the first version small and document any
assumptions.

Before coding, decide which status values are valid, which fields are
required, how updates work, and what the API returns when a task does
not exist or the request is invalid.

## 4. API behaviour

Use resource-oriented URLs and standard HTTP methods. One conventional
endpoint design is:

  Method     Endpoint        Purpose
  ---------- --------------- -------------------
  `POST`     `/tasks`        Create a task
  `GET`      `/tasks`        List tasks
  `GET`      `/tasks/{id}`   Retrieve one task
  `PUT`      `/tasks/{id}`   Update a task
  `DELETE`   `/tasks/{id}`   Delete a task

These are proposed routes; the assignment requires a RESTful API but
does not prescribe every route or response format.

The API should validate incoming data and return consistent errors. Use
suitable HTTP status codes, such as `201 Created` for successful
creation, `400 Bad Request` for invalid input, `404 Not Found` for a
missing task, and an appropriate success status for updates and deletes.
Document the exact response format in the OpenAPI specification and
README.

Authentication and authorization are not listed as requirements, so they
should not be added unless there is a clear reason or the interviewer
asks for them.

## 5. Proposed code structure

Keep the application small and separate responsibilities so each part is
easy to understand and test.

-   **HTTP handlers:** Receive requests, parse path parameters and JSON,
    call the service, and write HTTP responses.
-   **Service layer:** Contains task-related rules and coordinates
    operations. It should not depend on Gin request or response types.
-   **Repository layer:** Defines and implements database operations for
    tasks.
-   **Models and DTOs:** Represent stored task data and, where useful,
    API request and response shapes.
-   **Configuration:** Reads settings such as the database connection
    string and server port from environment variables.
-   **Application setup:** Connects the dependencies and starts the HTTP
    server.

A request should normally follow this path:

`HTTP request → Gin handler → task service → repository → PostgreSQL`

The response travels back through the same layers.

This separation is useful because the service logic can be tested
without starting an HTTP server, while handler tests can focus on HTTP
behaviour. Repository tests can check database-related behaviour
separately.

## 6. Data storage

PostgreSQL is the source of truth for task data. The repository should
use parameterized queries or a suitable database library rather than
building SQL by concatenating user input.

Plan how the database schema is created and updated. A migration is
preferable to relying on undocumented manual database setup. Handle
database errors explicitly and avoid returning raw internal database
errors to API clients.

The service should read database configuration from the environment
rather than hard-coding credentials. Docker Compose should provide the
local database configuration and connect the API to PostgreSQL using the
Compose service name.

## 7. Testing approach

Use a test-first workflow where practical:

1.  Write a test for the expected behaviour.
2.  Run it and confirm that it fails for the intended reason.
3.  Implement the smallest change that makes it pass.
4.  Refactor while keeping the tests green.

Unit tests should cover service rules, validation, and error cases. Mock
the repository or database-facing interface when testing logic that does
not need a real database.

Integration tests should verify the parts that need to work together,
especially HTTP handling and PostgreSQL persistence. A mock alone cannot
prove that SQL queries, migrations, constraints, or database
interactions work against PostgreSQL.

Measure coverage with Go's coverage tools and aim for at least 70%.
Coverage is a useful signal, but it does not prove that the tests check
meaningful behaviour. Include success cases, invalid input, missing
records, and relevant dependency failures.

## 8. Running and operating the service

The project should be easy for another developer to start locally.
Docker Compose should run the API and PostgreSQL, and the README should
explain the required environment variables and startup command.

The Docker image should use a multi-stage build so the final runtime
image does not need the full Go build environment. The application
should handle startup failures clearly, close resources when shutting
down, and avoid assuming that PostgreSQL is ready just because its
container has started.

Expose the Prometheus metrics endpoint and document the metrics. Request
counts and latency should be recorded at the HTTP layer. Task counts
need a clearly defined meaning---for example, the current number of
stored tasks---because a gauge, counter, and database query represent
different things. Add simple tracing and explain how to enable or
inspect it.

## 9. Documentation and deliverables

The repository should contain:

-   The complete Go source code and meaningful Git commits.
-   Dockerfile and Docker Compose configuration.
-   OpenAPI specification, such as `openapi.yaml` or `swagger.json`.
-   A README with prerequisites, startup steps, curl examples, request
    and response formats, test commands, coverage instructions,
    architecture notes, and trade-offs.
-   A brief architecture diagram if time allows.

Documentation should match the behaviour of the implemented service.
Update the examples and API specification whenever the actual routes or
response formats change.

## 10. Decisions and trade-offs

Keep the first version focused on the required scope. Choose the
simplest design that meets the requirements and explain the reasons for
it.

Before adding optional features, consider their cost:

-   **Redis:** Adds another dependency and cache invalidation problems.
    Add it only after the database-backed API works.
-   **Pagination and filtering:** Useful for larger task lists, but
    require defined query parameters, validation, ordering, and response
    metadata.
-   **Extra abstractions:** Interfaces are useful at boundaries that
    need mocking or interchangeable implementations, but unnecessary
    layers can make a small project harder to follow.
-   **Tracing and metrics:** Implement enough to demonstrate useful
    operational visibility without building a full monitoring platform.

Record the important choices and their limitations in the README. Be
ready to explain what problem each component solves, how it is tested,
and what you would improve if the service grew.

## 11. Completion criteria

The project is ready to submit when:

-   All required task operations work through the REST API.
-   Task data persists in PostgreSQL.
-   Invalid requests and missing tasks produce consistent responses.
-   Unit and integration tests run successfully and coverage is at least
    70%.
-   The API and database can be started using Docker Compose.
-   The OpenAPI documentation matches the implementation.
-   Prometheus metrics and basic tracing are available and documented.
-   The README explains how to run, test, and understand the service.
-   The implementation is small enough that its structure and design
    choices can be explained clearly during the interview.
