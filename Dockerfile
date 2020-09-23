# Use alpine image for the actual deployment
FROM alpine:latest

# Create app directory
WORKDIR /usr/src/app

# Copy the app
COPY bin/main .
COPY configs configs

# Add missing certificates
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
RUN chmod 755 ./main
# Bind the app port
EXPOSE 8092

# Start the app
CMD [ "./main" ]