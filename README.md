# Order Management System (OMS) in Go

A clean, production-ready Order Management System built with **Go**, **PostgreSQL**, **Redis**, and **Docker Compose**.

## System Architecture

- **Go (Chi Router)**: RESTful JSON API for products, inventory stock, orders, and order status transitions.
- **PostgreSQL 16**: Primary relational database storing `products`, `orders`, and `order_items` with transactional integrity.
- **Redis 7**: High-performance caching layer for catalog data & distributed locking key mechanism during order placement to prevent race conditions & double-selling.
- **Docker Compose**: Single-command container orchestration for local development and testing.

---

## Getting Started

### Prerequisites
- [Docker](https://www.docker.com/) & Docker Compose installed.
- (Optional) [Go 1.22+](https://go.dev/) if running without Docker.

### 1. Launch with Docker Compose

Run the following command in the project root:

```bash
docker compose up --build -d
```

To view real-time application logs:

```bash
docker compose logs -f app
```

To stop all containers:

```bash
docker compose down
```

---

## API Reference & Testing via cURL

The API runs at `http://localhost:8080`.

### 1. Health Check

```bash
curl http://localhost:8080/health
```

**Expected Response**:
```json
{
  "success": true,
  "message": "Order Management System API is healthy"
}
```

---

### 2. List Products (Cached via Redis)

```bash
curl http://localhost:8080/api/products
```

**Response**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "Wireless Noise-Canceling Headphones",
      "description": "High fidelity audio headphones with 30-hour battery life",
      "price": 199.99,
      "stock": 50
    },
    {
      "id": 2,
      "name": "Mechanical Gaming Keyboard",
      "description": "RGB tactile mechanical keyboard with custom switches",
      "price": 129.50,
      "stock": 30
    }
  ]
}
```

---

### 3. Create a New Product

```bash
curl -X POST http://localhost:8080/api/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "USB-C Dual Docking Station",
    "description": "4K dual monitor output with 100W power delivery",
    "price": 89.99,
    "stock": 25
  }'
```

---

### 4. Place an Order (Deducts Inventory & Acquired Redis Lock)

```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_name": "Alice Smith",
    "customer_email": "alice@example.com",
    "items": [
      {
        "product_id": 1,
        "quantity": 2
      },
      {
        "product_id": 2,
        "quantity": 1
      }
    ]
  }'
```

**Response**:
```json
{
  "success": true,
  "message": "Order created successfully",
  "data": {
    "id": 1,
    "customer_name": "Alice Smith",
    "customer_email": "alice@example.com",
    "total_amount": 529.48,
    "status": "PENDING",
    "items": [
      {
        "id": 1,
        "order_id": 1,
        "product_id": 1,
        "product_name": "Wireless Noise-Canceling Headphones",
        "quantity": 2,
        "unit_price": 199.99,
        "subtotal": 399.98
      },
      {
        "id": 2,
        "order_id": 1,
        "product_id": 2,
        "product_name": "Mechanical Gaming Keyboard",
        "quantity": 1,
        "unit_price": 129.5,
        "subtotal": 129.5
      }
    ]
  }
}
```

---

### 5. List All Orders

```bash
curl http://localhost:8080/api/orders
```

---

### 6. Get Order Details by ID

```bash
curl http://localhost:8080/api/orders/1
```

---

### 7. Update Order Status

Supported statuses: `PENDING`, `PROCESSING`, `COMPLETED`, `CANCELLED`.

> Note: Updating an order status to `CANCELLED` automatically restores the deducted inventory stock back to PostgreSQL catalog and clears Redis cache.

#### Mark as Processing:
```bash
curl -X PATCH http://localhost:8080/api/orders/1/status \
  -H "Content-Type: application/json" \
  -d '{"status": "PROCESSING"}'
```

#### Mark as Completed:
```bash
curl -X PATCH http://localhost:8080/api/orders/1/status \
  -H "Content-Type: application/json" \
  -d '{"status": "COMPLETED"}'
```

#### Cancel Order (Restores Stock):
```bash
curl -X PATCH http://localhost:8080/api/orders/1/status \
  -H "Content-Type: application/json" \
  -d '{"status": "CANCELLED"}'
```

---

## Directory Structure

```
├── cmd/
│   └── server/
│       └── main.go           # Application entry point
├── internal/
│   ├── cache/
│   │   └── redis.go          # Redis client, caching, and distributed locking
│   ├── config/
│   │   └── config.go         # Environment configuration parser
│   ├── database/
│   │   └── postgres.go       # PostgreSQL database connection pool
│   ├── handlers/
│   │   └── handlers.go       # REST API endpoints & JSON serialization
│   ├── models/
│   │   └── models.go         # Data structs and DTOs
│   ├── repository/
│   │   ├── order_repository.go    # DB queries for Orders & Order Items
│   │   └── product_repository.go  # DB queries for Products & Stock
│   └── service/
│       └── order_service.go       # Business logic (atomic locks, state machine)
├── migrations/
│   └── init.sql              # Database DDL & sample seed data
├── Dockerfile                # Multi-stage Docker build
├── docker-compose.yml        # Services orchestrator (Go App, Postgres, Redis)
├── go.mod                    # Module dependencies
└── README.md                 # Project documentation
```
