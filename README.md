# Trading Ace

Trading Ace is a backend application designed to manage a reward campaign for Uniswap users. The platform incentivizes user participation through a point-based system tied to their trading activities.

## Project Overview

This campaign runs for 4 weeks and offers two main tasks for users to earn points:

### 1. Onboarding Task
- Users need to swap at least 1000u on Uniswap
- Upon completion, users receive 100 points immediately
- This is a one-time task

### 2. Share Pool Task
- Points are awarded based on user's proportion of swap volume (USD) in the target pool
- Users share from a weekly pool of 10,000 points
- Requires completion of the onboarding task to be eligible
- Target Pool: Uniswap V2 WETH/USDC
  - Pool Address: `0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc`

### Key Features
- Real-time event monitoring of Uniswap V2 WETH/USDC pool
- Points calculation and distribution system
- Campaign duration management with configurable start time
- Support for both historical (backtest mode) and real-time events
- WebSocket support for real-time updates
- RESTful APIs for user task status and points history
- Leaderboard system based on accumulated points

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
└── docker-compose.yml
```

## Prerequisites

- Go 1.22.4 or higher
- PostgreSQL 13
- Docker and Docker Compose (for local development)

## Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/SIMPLYBOYS/trading_ace.git
   cd trading_ace
   ```

2. Set up environment variables:
   ```bash
   # Required for Ethereum connection
   export INFURA_PROJECT_ID="your_infura_project_id"
   
   # Required for database connection
   export DB_HOST="localhost"
   export DB_PORT="5432"
   export DB_USER="user"
   export DB_PASSWORD="password"
   export DB_NAME="tradingace"
   ```

3. Start the PostgreSQL database using Docker Compose:
   ```bash
   docker-compose up -d
   ```

4. Build and run the application:
   ```bash
   go build -v ./cmd/tradingace
   ./tradingace
   ```

   The application should now be running and accessible at `http://localhost:8080`.

## API Documentation

### 1. Get User Tasks Status
Retrieves the status of all tasks for a specific user.

**Endpoint:** `GET /user/:address/tasks`

**Example Request:**
```bash
GET /user/0x1234567890abcdef1234567890abcdef12345678/tasks
```

**Example Response:**
```json
{
  "onboarding": {
    "completed": true,
    "amount": 1500.50,
    "points": 100
  },
  "sharePool": {
    "completed": true,
    "amount": 750.25,
    "points": 50,
    "eligible": true
  },
  "campaign": {
    "startTime": "2024-03-01T00:00:00Z",
    "endTime": "2024-03-29T00:00:00Z",
    "isActive": true
  }
}
```

### 2. Get User Points History
Retrieves the points history for a specific user.

**Endpoint:** `GET /user/:address/points`

**Example Response:**
```json
{
  "history": [
    {
      "points": 100,
      "reason": "Onboarding Task",
      "timestamp": "2024-03-15T14:30:00Z"
    },
    {
      "points": 50,
      "reason": "Weekly Share Pool",
      "timestamp": "2024-03-14T00:00:00Z"
    }
  ],
  "totalPoints": 150
}
```

### 3. Get Current Ethereum Price
Retrieves the current Ethereum price.

**Endpoint:** `GET /ethereum/price`

**Example Response:**
```json
{
  "price": 2034.56,
  "timestamp": "2024-03-15T15:00:00Z"
}
```

### 4. Get Leaderboard
Retrieves the current leaderboard and campaign status.

**Endpoint:** `GET /leaderboard`

**Example Response:**
```json
{
  "leaderboard": [
    {
      "address": "0x1234...5678",
      "points": 1500,
      "rank": 1
    },
    {
      "address": "0xabcd...efgh",
      "points": 1200,
      "rank": 2
    }
  ],
  "campaign": {
    "startTime": "2024-03-01T00:00:00Z",
    "endTime": "2024-03-29T00:00:00Z",
    "isActive": true,
    "totalParticipants": 156
  }
}
```

## WebSocket Support

The application supports real-time updates via WebSocket connections. Clients can connect to the `/ws` endpoint to receive updates on:

### Event Types

1. **Swap Event:**
```json
{
  "type": "swap_event",
  "event": {
    "txHash": "0x1234...",
    "sender": "0xabcd...",
    "amount": 1000.50,
    "usdValue": 1500.75,
    "points": 15,
    "timestamp": "2024-03-15T15:30:00Z"
  }
}
```

2. **User Points Update:**
```json
{
  "type": "user_points_update",
  "address": "0x1234...",
  "points": 50,
  "reason": "Weekly Share Pool",
  "timestamp": "2024-03-15T00:00:00Z"
}
```

3. **Leaderboard Update:**
```json
{
  "type": "leaderboard_update",
  "leaderboard": [
    {
      "address": "0x1234...",
      "points": 1500,
      "rank": 1
    }
  ]
}
```

### WebSocket Connection Example

```javascript
const socket = new WebSocket('ws://localhost:8080/ws');

socket.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};

socket.onclose = function(event) {
    console.log('WebSocket connection closed:', event);
};

socket.onerror = function(error) {
    console.error('WebSocket error:', error);
};
```

## Error Responses

All endpoints use standard HTTP status codes and return errors in this format:

```json
{
  "error": "Error message",
  "details": "Detailed error information",
  "code": "ERROR_CODE"
}
```

Common status codes:
- 200: Success
- 400: Bad Request
- 404: Not Found
- 500: Internal Server Error

## Docker Configuration

The project uses Docker Compose for the PostgreSQL database service:

```yaml
version: '3.8'
services:
  db:
    image: postgres:13
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: tradingace
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"

volumes:
  postgres_data:
```

## Testing

Run the tests with the following command:

```bash
go test ./... -cover
```

The test coverage requirements is at least 50%.