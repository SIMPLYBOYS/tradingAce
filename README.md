# Trading Ace

Trading Ace is a Go-based application designed to manage a campaign for Uniswap's users. The campaign consists of tasks for users to collect points, with rewards based on accumulated points at the end of the campaign.

## Features

- Onboarding Task: Users swap at least 1000u to get 100 points immediately.
- Share Pool Task: Points awarded based on the proportion of user's swap volume among all users on the target pool.
- Real-time processing of swap events from the Ethereum blockchain.
- Weekly calculation of share pool points.
- RESTful API for retrieving user tasks status, points history, and Ethereum price.

## Prerequisites

- Go 1.18 or later
- Docker and Docker Compose
- Infura account (for Ethereum blockchain interaction)

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/SIMPLYBOYS/tradingAce.git

   cd trading-ace
   ```

2. Install Go dependencies:
   ```
   go mod download
   ```

## Configuration

The application uses environment variables for configuration. The following environment variable is required:

- `INFURA_PROJECT_ID`: Your Infura project ID

Set it in your environment before running the application:

```
export INFURA_PROJECT_ID=your_project_id_here
```

Note: For Windows, use `set` instead of `export`.

Database configuration is handled through Docker Compose and doesn't require manual setup.

## Running the Application

### Development Environment

1. Start the PostgreSQL database using Docker Compose:
   ```
   docker-compose up db
   ```

2. In a new terminal, run the application:
   ```
   go run .
   ```

3. The application will start and listen on port 8080.

### Production-like Environment

1. Start the PostgreSQL database using Docker Compose:
   ```
   docker-compose up -d db
   ```

2. Build the Go application:
   ```
   go build -o trading-ace
   ```

3. Run the compiled application:
   ```
   ./trading-ace
   ```

Note: For a full containerized deployment, additional configuration would be needed in the `docker-compose.yml` file to include the application service.

## API Endpoints

- GET `/user/:address/tasks`: Get user tasks status
- GET `/user/:address/points`: Get user points history
- GET `/ethereum/price`: Get current Ethereum price

## Docker Configuration

The current `docker-compose.yml` file is configured to set up the PostgreSQL database. Here's an overview:

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

This configuration sets up a PostgreSQL database with the following details:
- Database name: tradingace
- User: user
- Password: password

   This command will build the Trading Ace application image and start both the application and the PostgreSQL database.

4. The application will be accessible at `http://localhost:8080`.

5. To stop the services, use:
   ```
   docker-compose down
   ```

Note: For production deployments, it's recommended to use a `.env` file or environment variables instead of hardcoding values in the `docker-compose.yml` file.

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
- `*_test.go`: Test files for respective packages
- `docker-compose.yml`: Docker configuration file

## Development Notes

- The application uses the Uniswap V2 WETH/USDC pool for tracking swap events.
- Ethereum interaction is done through Infura, ensure your Infura project has sufficient capacity for the expected load.
- The campaign runs for 4 weeks, with weekly share pool point calculations.
- Ensure proper error handling and logging in production environments.