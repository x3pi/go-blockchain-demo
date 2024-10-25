# Dockerfile
FROM golang:1.23.1

# Thiết lập thư mục làm việc
WORKDIR /app

# Sao chép mã nguồn vào container
COPY . .

# Cài đặt các phụ thuộc
RUN go mod tidy

# Biên dịch ứng dụng
RUN go build -o myapp .

# Chạy ứng dụng
CMD ["./myapp"]
