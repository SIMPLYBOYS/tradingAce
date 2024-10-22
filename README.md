# Trading Ace

Trading Ace is a backend application for managing a user rewards campaign based on Uniswap trading activity.

## Project Structure

```
project/
├── cmd/
│   └── tradingace/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── handlers.go
│   │   └── router.go
│   ├── db/
│   │   ├── models.go
│   │   └── operations.go
│   ├── ethereum/
│   │   ├── client.go
│   │   └── events.go
│   └── websocket/
│       └── manager.go
├── pkg/
│   └── logger/
│       └── logger.go
├── go.mod
├── go.sum
└── docker-compose.yml
```

## Prerequisites

- Go 1.16 or higher
- PostgreSQL 13
- Docker and Docker Compose (for local development)

## Getting Started

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/trading_ace.git
   cd trading_ace
   ```

2. Set up the environment:
   Create a `.env` file in the root directory with the following content:
   ```
   INFURA_PROJECT_ID=your_infura_project_id
   POSTGRES_DB=tradingace
   POSTGRES_USER=user
   POSTGRES_PASSWORD=password
   ```

3. Start the application using Docker Compose:
   ```
   docker-compose up -d
   ```

   This will start both the PostgreSQL database and the Trading Ace application.

4. The application should now be running and accessible at `http://localhost:8080`.

## API Endpoints

- `GET /user/:address/tasks`: Get user tasks status
- `GET /user/:address/points`: Get user points history
- `GET /ethereum/price`: Get current Ethereum price
- `GET /leaderboard`: Get the current leaderboard
- `GET /ws`: WebSocket endpoint for real-time updates

## WebSocket Support

Trading Ace supports real-time updates via WebSocket connections. Clients can connect to the `/ws` endpoint to receive updates on:

- Swap events
- User points updates
- Leaderboard changes

To connect to the WebSocket from a client:

```javascript
const socket = new WebSocket('ws://localhost:8080/ws');

socket.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};
```

## Docker Configuration

The project uses Docker Compose for easy setup and deployment. The `docker-compose.yml` file defines two services:

1. `db`: PostgreSQL database
2. `app`: Trading Ace application

To build and run the application using Docker Compose:

1. Ensure you have Docker and Docker Compose installed on your system.

2. Build the Docker images:
   ```
   docker-compose build
   ```

3. Start the services:
   ```
   docker-compose up -d
   ```

4. To stop the services:
   ```
   docker-compose down
   ```

## Testing

Run the tests with the following command:

```
go test ./... -cover
```