# Build Stage
FROM golang:1.25 AS builder

WORKDIR /app

# Copy dependency files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Ensure the binary is statically compiled for Alpine by disabling CGO,
# and reduce binary size with linker flags.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o ticketpulse .

# Final Stage
FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget

# Create a non-root user for security
RUN adduser -D appuser
WORKDIR /home/appuser

# Copy the statically built binary from the builder stage
COPY --from=builder /app/ticketpulse .

# Copy templates and static assets required at runtime
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Expose the application port
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Switch to the non-root user
USER appuser

# Run the application
CMD ["./ticketpulse"]
