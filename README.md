# Distributed Realtime Chat & Call System

A high-performance microservices application showcasing Go monorepo architecture, gRPC inter-service communication, real-time WebSocket messaging, and database persistence.

---

## 1. Accessing the Application

When the application is running, you can access the frontend web interface directly via your browser:
*   **Web Address:** [http://localhost:8080](http://localhost:8080)

### Seeded Test Credentials
You can log in immediately using the pre-seeded accounts:
*   **Alice:** Username: `alice`, Password: `password123`
*   **Bob:** Username: `bob`, Password: `password123`

---

## 2. Command Reference

All commands must be executed from the directory containing `docker-compose.yml`.

### Start the Application (Run in Background)
To build and start the entire service cluster:
```bash
docker compose up -d --build
```

### Stop the Application (Free Ports)
If you no longer want to run the application and want to shut down the ports, run:
```bash
docker compose stop
```
*Note: This halts the containers and frees ports `8080` and `5432` but preserves the database volumes.*

### Clean Takedown (Teardown & Remove Data)
To completely stop and remove containers, networks, and database volumes:
```bash
docker compose down -v
```

---

## 3. Port Mapping Reference
The following local ports are exposed to your host machine:
*   **Gateway Service (HTTP/WS):** Port `8080`
*   **PostgreSQL Database:** Port `5432`
*   **Auth Service (gRPC):** Port `50051`
*   **Chat Service (gRPC):** Port `50052`
*   **Call Service (gRPC):** Port `50053`
