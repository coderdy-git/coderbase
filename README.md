# Coderbase Studio 🚀
> **Lightweight self-hosted Backend-as-a-Service (BaaS) built in Go.**

Coderbase is an ultra-lightweight, high-performance alternative to Supabase and Firebase. It is specifically designed to run efficiently on low-spec, budget VPS hosting nodes (e.g. 512MB - 1GB RAM) using SQLite locally or PostgreSQL in production.

---

## ✨ Features

- **Multi-Tenant Isolation**: Supports multiple projects and users inside a single database instance using isolated prefix table mapping.
- **Auto-CRUD REST API**: Instantly generate clean API endpoints (`GET`, `POST`, `PATCH`, `DELETE`) for any custom table schema.
- **Visual Schema & Table Editor**: Manage tables and create columns (TEXT, INTEGER, BOOLEAN, TIMESTAMP, JSONB) directly from the visual dashboard.
- **Import JSON Tool**: Import JSON datasets (single objects or arrays) into database tables instantly through the UI.
- **Security Policies (RLS)**: Fine-grained visual Row-Level Security editor supporting Public Access and authenticated Owner-Only access.
- **User Authentication**: Built-in signup, login, password encryption via `bcrypt`, and JWT token verification.
- **Realtime Pub/Sub**: WebSocket broadcaster that broadcasts database changes (`INSERT`, `UPDATE`, `DELETE`) to clients instantly.
- **Interactive API Docs**: Dynamically generates OpenAPI 3.0 specs and displays an interactive Swagger UI customized to each project's schema.
- **Sleek Admin Dashboard**: A premium dark-mode console (Emerald theme) with inline logs, traffic latency tracking, and copy-to-clipboard conveniences.
- **Secure Console**: Locked by a cookie session-based login page to prevent unauthorized access in production.

---

## 🛠️ Technology Stack

- **Backend**: Go (Golang), Chi Router, WebSocket (Gorilla-style Pub/Sub), `golang.org/x/crypto` (bcrypt)
- **Database Engine**:
  - **Local/Development**: SQLite (`gobaas.db` fallback)
  - **Production/VPS**: PostgreSQL
- **Frontend Dashboard**: HTML5, Vanilla CSS, TailwindCSS (CDN), interactive custom JS tabs and tooltips.

---

## 📂 Project Structure

```text
gobaas/
├── auth/           # JWT & Bcrypt Authentication Module
├── crud/           # Dynamic Auto-CRUD REST Engine
├── dashboard/      # SSR Studio Console & HTML Templates
├── db/             # SQLite & PostgreSQL database connectors
├── middleware/     # API Key & JWT Loggers
├── policy/         # Row-Level Security (RLS) Policy compiler
├── realtime/       # WebSockets Pub/Sub Broadcaster
├── schema/         # DDL execution, dynamic columns, & OpenAPI generator
├── main.go         # Core entrypoint
├── Dockerfile      # Multi-stage production Docker image
└── docker-compose.yml
```

---

## 🚀 Getting Started

### 1. Requirements
Ensure you have Go installed:
```bash
go version # (Go 1.20 or newer is recommended)
```

### 2. Run Locally
By default, if no Postgres credentials are set, Coderbase automatically provisions a local SQLite database file named `gobaas.db` in the workspace root.

```bash
# Run the server
go run main.go
```

The server will spin up on port `8080`.
* **Studio Console**: Access [http://localhost:8080/dashboard](http://localhost:8080/dashboard)
* **Default Login credentials**:
  * **Username**: `admin`
  * **Password**: `admin123`

---

## 🔑 Environment Variables

To configure Coderbase for staging or production, inject the following environment variables:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | Listening port for the HTTP server | `8080` |
| `DATABASE_URL` | PostgreSQL connection string. If empty, uses SQLite. | *None* |
| `JWT_SECRET` | Secret string for signing auth tokens | `coderbase_secret` |
| `ADMIN_USERNAME` | Login username for Coderbase Studio | `admin` |
| `ADMIN_PASSWORD` | Login password for Coderbase Studio | `admin123` |

---

## 🐳 VPS Deployment (Docker Compose)

Deploy Coderbase to your cloud VPS instance using the included files.

1. Create a `docker-compose.yml` file:
```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - DATABASE_URL=postgres://postgres:mysecretpassword@db:5432/coderbase?sslmode=disable
      - JWT_SECRET=change_me_to_a_secure_long_secret
      - ADMIN_USERNAME=super_admin
      - ADMIN_PASSWORD=super_secret_password_123
    depends_on:
      - db

  db:
    image: postgres:15-alpine
    restart: always
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=mysecretpassword
      - POSTGRES_DB=coderbase
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

2. Start the services:
```bash
docker-compose up -d --build
```

---

## 🔌 API Reference Quickstart

All client requests must include your project's unique API Key in the headers.

### 1. Database CRUD
- **Fetch Records**:
  `GET http://localhost:8080/api/v1/tables/{table_name}`
  - *Header*: `X-API-Key: gb_your_project_api_key`
- **Insert Record**:
  `POST http://localhost:8080/api/v1/tables/{table_name}`
  - *Header*: `X-API-Key: gb_your_project_api_key`
  - *Body*: (JSON matching your custom columns)
- **Update Record**:
  `PATCH http://localhost:8080/api/v1/tables/{table_name}/{id}`
  - *Header*: `X-API-Key: gb_your_project_api_key`
- **Delete Record**:
  `DELETE http://localhost:8080/api/v1/tables/{table_name}/{id}`
  - *Header*: `X-API-Key: gb_your_project_api_key`

### 2. Authentication (Users)
- **Sign Up User**:
  `POST http://localhost:8080/api/auth/signup`
  - *Header*: `X-API-Key: gb_your_project_api_key`
  - *Body*: `{"email": "user@example.com", "password": "secure_password"}`
- **Login User**:
  `POST http://localhost:8080/api/auth/login`
  - *Header*: `X-API-Key: gb_your_project_api_key`
  - *Body*: `{"email": "user@example.com", "password": "secure_password"}`
  - *Response*: `{"token": "eyJhbG..."}` (use as JWT Token)

### 3. Realtime Updates
Subscribe to database changes dynamically using WebSockets:
```javascript
const ws = new WebSocket("ws://localhost:8080/api/v1/realtime");

// Listen to broadcast events
ws.onmessage = (event) => {
    const payload = JSON.parse(event.data);
    console.log("Realtime event received:", payload);
    // Payload contains: { project_id, table_name, action (INSERT/UPDATE/DELETE), record }
};
```

---

## 📄 License
This project is open-source and available under the [MIT License](LICENSE).
