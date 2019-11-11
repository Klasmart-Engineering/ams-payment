# Use the latest Go 1.X image for building
FROM golang:1 as builder

# Add variables for credentials
ARG BITBUCKET_USER
ENV BITBUCKET_USER=$BITBUCKET_USER
ARG BITBUCKET_APP_PASS
ENV BITBUCKET_APP_PASS=$BITBUCKET_APP_PASS

# Enable Go modules
ENV GO111MODULE=on

# Create app directory
WORKDIR /usr/src/appbuild

# Copy the source files
COPY src src
COPY go.mod go.mod
COPY go.sum go.sum

# Download all the dependencies and build
RUN git config --global url."https://${BITBUCKET_USER}:${BITBUCKET_APP_PASS}@bitbucket.org".insteadOf "https://bitbucket.org" && \
    go get ./... && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/handler ./src

# Use alpine image for the actual deployment
FROM alpine:latest

# Create app directory
WORKDIR /usr/src/app

# Copy the app
COPY --from=builder /usr/src/appbuild/bin/handler bin/
COPY configs configs

# Add missing certificates
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

# Bind the app port
EXPOSE 8092

# Start the app
ENTRYPOINT [ "bin/handler" ]
