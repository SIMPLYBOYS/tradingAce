package errors

import "fmt"

type DatabaseError struct {
	Operation string
	Err       error
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("database error during %s: %v", e.Operation, e.Err)
}

type NotFoundError struct {
	Resource   string
	Identifier string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.Identifier)
}

type EthereumError struct {
	Operation string
	Err       error
}

func (e *EthereumError) Error() string {
	return fmt.Sprintf("ethereum error during %s: %v", e.Operation, e.Err)
}

type APIError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s - %v", e.StatusCode, e.Message, e.Err)
}

type WebSocketError struct {
	Operation string
	Err       error
}

func (e *WebSocketError) Error() string {
	return fmt.Sprintf("WebSocket error during %s: %v", e.Operation, e.Err)
}
