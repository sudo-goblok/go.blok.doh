# menggunakan base image Golang berbasis Alpine
FROM golang:1.24.2-alpine

# Install tools tambahan
RUN apk add --no-cache \
    ca-certificates \
    bind-tools \
    curl \
    iputils

# Set working directory di dalam container
WORKDIR /app

# Copy semua isi folder src ke dalam container
COPY ./src/ ./

# Jalankan go mod tidy
RUN go mod tidy

# Build aplikasi
RUN go build -o app .

# Salin config.yaml ke dalam image
RUN mkdir -p /app/config
#COPY ./src/config/config.yaml /app/config/config.yaml
COPY ./config.default.yaml . 

#  entrypoint script untuk maintenance config.yaml 
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose port yang digunakan
EXPOSE 53

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./app"]
