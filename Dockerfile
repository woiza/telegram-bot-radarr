FROM golang AS builder

# create a working directory inside the image
WORKDIR /source

# copy Go modules and dependencies to image
COPY . .

# download Go modules and dependencies
RUN go mod download

# compile application
RUN CGO_ENABLED=0 go build -o /app/bot /source/cmd/bot/main.go

# Now copy it into a base image.
FROM alpine
COPY --from=builder /app/bot /app/bot
CMD ["/app/bot"]
