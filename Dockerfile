FROM golang AS builder

# Set the working directory inside the Docker image
WORKDIR /source

# Copy the 'cmd' and 'pkg' directories and the Go module files into the Docker image
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY go.mod go.sum ./

# Download the Go modules
RUN go mod download

# Add the -ldflags '-w -s' flags to reduce the size of the binary
RUN CGO_ENABLED=0 go build -a -ldflags '-w -s' -o /app/bot ./cmd/bot/main.go

# Now copy it into a base image.
FROM alpine

# Create a group and user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Tell docker that all future commands should run as the appuser user
USER appuser

COPY --from=builder /app/bot /app/bot
CMD ["/app/bot"]