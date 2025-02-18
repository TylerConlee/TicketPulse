# Build Stage
FROM golang:1.23 AS builder

WORKDIR /app

# Copy dependency files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Ensure the binary is statically compiled for Alpine by disabling CGO,
# and reduce binary size with linker flags. Also, explicitly build the package in the current directory.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o ticketpulse .

# Final Stage
FROM alpine:3.18

# Create a non-root user for security
RUN adduser -D appuser
WORKDIR /home/appuser

# Copy the statically built binary from the builder stage
COPY --from=builder /app/ticketpulse .

# Expose the application port
EXPOSE 8080

# Switch to the non-root user
USER appuser

# Run the application
CMD ["./ticketpulse"]
