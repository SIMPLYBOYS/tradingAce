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

## Configuration

During the development stage, environment variables are hardcoded in the application for simplicity. Here's how it's set up:

1. Infura Project ID: The Infura URL is hardcoded in the `ethereum.go` file. For example:
   ```go
   const InfuraURL = "https://mainnet.infura.io/v3/YOUR_PROJECT_ID"
   ```
   Replace `YOUR_PROJECT_ID` with your actual Infura project ID.

2. Database Configuration: The database connection string is hardcoded in the `db.go` file. For example:
   ```go
   const connStr = "host=localhost port=5432 user=your_username password=your_password dbname=tradingace sslmode=disable"
   ```
   Update this string with your local PostgreSQL configuration.

3. Campaign Settings: The campaign configuration is stored in the database. You can modify the default values in the relevant functions in `db.go`.

Note: For production deployments, it's recommended to use environment variables for sensitive information. The code can be modified to read from environment variables instead of using hardcoded values.

## Running the Application

1. Ensure your PostgreSQL database is running and accessible.

2. Build the application:
   ```
   go build
   ```

3. Run the application:
   ```
   ./trading-ace
   ```

4. The application will start and listen on port 8080.

## API Endpoints

- GET `/user/:address/tasks`: Get user tasks status
- GET `/user/:address/points`: Get user points history

## Docker Deployment

For Docker deployment, we use Docker Compose with hardcoded environment variables in the development stage:

1. Ensure you have Docker and Docker Compose installed on your system.

2. The `docker-compose.yml` file should already contain the necessary environment variables. For example:
   ```yaml
   version: '3.8'
   services:
     app:
       build: .
       ports:
         - "8080:8080"
       environment:
         - INFURA_PROJECT_ID=your_hardcoded_project_id
         - DB_HOST=db
         - DB_USER=your_db_user
         - DB_PASSWORD=your_db_password
         - DB_NAME=tradingace
       depends_on:
         - db
     db:
       image: postgres:13
       environment:
         - POSTGRES_DB=tradingace
         - POSTGRES_USER=your_db_user
         - POSTGRES_PASSWORD=your_db_password
       volumes:
         - postgres_data:/var/lib/postgresql/data

   volumes:
     postgres_data:
   ```

3. Use Docker Compose to build and start the services:
   ```
   docker-compose up --build
   ```

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