# Dockerfile
FROM golang:1.23.2

# Install netcat for healthcheck
RUN apt-get update && apt-get install -y netcat-openbsd

# Thiết lập thư mục làm việc
WORKDIR /app

# Sao chép mã nguồn vào container
COPY . .

# Cài đặt các phụ thuộc
RUN go mod tidy

# Biên dịch ứng dụng
RUN go build -o node ./cmd/node/node.go

# Chạy ứng dụng
CMD ["./node"]
