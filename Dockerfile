# Use the official Golang image as the builder
FROM golang:1.23 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application binary
RUN go build -o /ticketpulse

# Use a minimal base image to run the application
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /ticketpulse .

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["./ticketpulse"]
