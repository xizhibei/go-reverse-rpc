# Basic Client/Server Example

This example demonstrates a simple implementation of a reverse RPC server and client using the go-reverse-rpc framework. The server provides basic math operations that can be called by the client.

## Features

- Basic math operations (add, subtract, multiply, divide)
- Error handling demonstration
- Timeout handling
- Telemetry integration
- Rate limiting example

## Prerequisites

- Go 1.20 or later
- Running MQTT broker (e.g., Mosquitto)

## Running the Example

1. Start your MQTT broker (if not already running):
```bash
# Using Mosquitto
mosquitto -c /usr/local/etc/mosquitto/mosquitto.conf
```

2. Start the server:
```bash
go run server/main.go
```

3. In another terminal, run the client:
```bash
go run client/main.go
```

## Code Structure

- `server/main.go`: Implements the RPC server with math operations
- `client/main.go`: Implements the RPC client that calls the server methods

## Error Handling

The example demonstrates proper error handling for cases such as:
- Division by zero
- Invalid parameters
- Timeout scenarios
- Connection issues

## Metrics and Telemetry

The example includes basic metrics collection:
- Request duration
- Error counts
- Request rates

View metrics in the logs or through OpenTelemetry exports.
