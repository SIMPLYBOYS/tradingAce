# Trading Ace

Trading Ace is a Go-based application designed to manage a campaign for Uniswap's users. The campaign consists of tasks for users to collect points, with rewards based on accumulated points at the end of the campaign.

## Features

- Onboarding Task: Users swap at least 1000u to get 100 points immediately.
- Share Pool Task: Points awarded based on the proportion of user's swap volume among all users on the target pool.
- Real-time processing of swap events from the Ethereum blockchain.
- Weekly calculation of share pool points.
- RESTful API for retrieving user tasks status and points history.

## Prerequisites

- Go 1.18 or later
- PostgreSQL
- Docker (optional, for containerized deployment)

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/SIMPLYBOYS/trading-ace.git
   cd trading-ace
   ```

2. Install dependencies:
   ```
   go mod download
   ```

3. Set up the PostgreSQL database:
   - Create a new database named `tradingace`
   - Update the database connection string in `db.go` if necessary

4. Run database migrations:
   ```
   go run migrations.go
   ```

## Configuration

- The campaign configuration can be set in the database or a config file.
- Ensure you have set the `INFURA_PROJECT_ID` environment variable for Ethereum network access.

## Running the Application

1. Build and run the application:
   ```
   go build
   ./trading-ace
   ```

2. The application will start and listen on port 8080.

## API Endpoints

- GET `/user/:address/tasks`: Get user tasks status
- GET `/user/:address/points`: Get user points history

## Docker Deployment

1. Build the Docker image:
   ```
   docker build -t trading-ace .
   ```

2. Run the container:
   ```
   docker run -p 8080:8080 -e INFURA_PROJECT_ID=your_project_id trading-ace
   ```

## Testing

Run the tests with:

```
go test -v ./...
```

For test coverage:

```
go test -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## Project Structure

- `main.go`: Entry point of the application
- `db.go`: Database operations
- `ethereum.go`: Ethereum-related operations
- `api.go`: API endpoint handlers
- `logger.go`: Logging utilities
- `migrations/`: SQL migration files