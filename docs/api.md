# Tài liệu API Blockchain

Tài liệu này mô tả các endpoint API có sẵn để tương tác với blockchain.

## URL Cơ sở

```
http://localhost:8080
```

## Các Endpoint

### Lấy Thông tin Tài khoản

Truy xuất thông tin tài khoản bao gồm số dư, khóa công khai và số nonce.

**Endpoint:** `GET /api/account/{address}`

**Tham số:**
- `address`: Địa chỉ Ethereum của tài khoản

**Phản hồi:**
```json
{
  "success": true,
  "data": {
    "address": "0xf780fbF9ff9952897425f302b362eDBb80447108",
    "balance": 1000,
    "publicKey": "0x048f1273da4d7c042caa74c4fe50443831875128a8ff7817c40f1211cdf6e65e63e5ce5139da1983946cb15a054d951559523d7121ae9d0314f5e187cc757b36e2",
    "nonce": 0
  }
}
```

**Ví dụ:**
```bash
curl http://localhost:8080/api/account/0xf780fbF9ff9952897425f302b362eDBb80447108
```

### Tạo Giao dịch

Tạo và phát tán một giao dịch mới vào mạng.

**Endpoint:** `POST /api/transaction`

**Nội dung yêu cầu:**
```json
{
  "from": "0xf780fbF9ff9952897425f302b362eDBb80447108",
  "to": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
  "amount": 100,
  "privateKeyHex": "khóa_riêng_của_bạn"
}
```

**Phản hồi:**
```json
{
  "success": true,
  "transaction": {
    "id": "0x1234567890abcdef...",
    "from": "0xf780fbF9ff9952897425f302b362eDBb80447108",
    "to": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
    "amount": 100,
    "timestamp": 1678901234
  }
}
```

**Ví dụ:**
```bash
curl -X POST http://localhost:8080/api/transaction \
  -H "Content-Type: application/json" \
  -d '{
    "from": "0xf780fbF9ff9952897425f302b362eDBb80447108",
    "to": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
    "amount": 100,
    "privateKeyHex": "khóa_riêng_của_bạn"
  }'
```

### Lấy Thông tin Giao dịch

Truy xuất thông tin về một giao dịch cụ thể.

**Endpoint:** `GET /api/transaction/{transactionId}`

**Tham số:**
- `transactionId`: Mã băm của giao dịch

**Phản hồi:**
```json
{
  "success": true,
  "data": {
    "id": "0x1234567890abcdef...",
    "from": "0xf780fbF9ff9952897425f302b362eDBb80447108",
    "to": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
    "amount": 100,
    "timestamp": 1678901234
  }
}
```

**Ví dụ:**
```bash
curl http://localhost:8080/api/transaction/0x1234567890abcdef...
```

### Lấy Block Cuối cùng

Truy xuất thông tin về block cuối cùng trong blockchain.

**Endpoint:** `GET /api/block/last`

**Phản hồi:**
```json
{
  "success": true,
  "data": {
    "index": 1,
    "version": 1,
    "merkleRoot": "0x1234567890abcdef...",
    "time": 1678901234,
    "signature": "0xabcdef1234567890...",
    "transactions": [
      "0x1234567890abcdef...",
      "0x9876543210fedcba..."
    ]
  }
}
```

**Ví dụ:**
```bash
curl http://localhost:8080/api/block/last
```

### Lấy Block Hiện tại

Truy xuất thông tin về block đang được xử lý.

**Endpoint:** `GET /api/block/current`

**Phản hồi:**
```json
{
  "success": true,
  "data": {
    "index": 2,
    "version": 1,
    "merkleRoot": "0x1234567890abcdef...",
    "time": 1678901234,
    "signature": "0xabcdef1234567890...",
    "transactions": [
      "0x1234567890abcdef...",
      "0x9876543210fedcba..."
    ]
  }
}
```

**Ví dụ:**
```bash
curl http://localhost:8080/api/block/current
```

## Phản hồi Lỗi

Tất cả các endpoint có thể trả về phản hồi lỗi theo định dạng sau:

```json
{
  "error": "Thông báo lỗi mô tả điều gì đã xảy ra"
}
```

Các mã trạng thái HTTP phổ biến:
- `200`: Thành công
- `201`: Đã tạo (cho yêu cầu POST)
- `400`: Yêu cầu không hợp lệ
- `404`: Không tìm thấy
- `405`: Phương thức không được phép
- `500`: Lỗi máy chủ nội bộ

## Tài khoản Test

Hệ thống được khởi tạo với ba tài khoản test:

1. Tài khoản 1:
   - Địa chỉ: 0xf780fbF9ff9952897425f302b362eDBb80447108
   - Khóa công khai: 0x048f1273da4d7c042caa74c4fe50443831875128a8ff7817c40f1211cdf6e65e63e5ce5139da1983946cb15a054d951559523d7121ae9d0314f5e187cc757b36e2
   - Số dư ban đầu: 1000
   - privateKeyHex: 6f389f57a5bca725595e26972d8db9023fa092fe60b5e332760de48ffadc3dac

2. Tài khoản 2:
   - Khóa công khai: 0x04127bae1dc0022eabf3fc16447d501fa45906d5127d116de654b6e93b0606ee9430552b8f458d905459396d581b9201b7745edf1d2f47a84aed5e063cc196942b
   - Số dư ban đầu: 1000
    - privateKeyHex: fdf0edfd86751d13e9ecbbf0c3a1b197cbccfa1ad5758aaeb22a54dc967fda9f

3. Tài khoản 3:
   - Khóa công khai: 0x04d53415bd1e6941e971c2eb1f9a4e964dd8cb043a5865f023f004915aef78d462d61072e9448ea334070cc26460c471a6c6ca9bc82461c02ddea5ca2bf7c170f6
   - Số dư ban đầu: 1000
  - privateKeyHex: 9ba58faba7490a9164c3d2737f073ddb755739a839afa976b119d9e6d9f13efd
```

## Ví dụ chuyển tiền từ tài khoản 1 sang tài khoản 2


```bash
    curl -X POST http://localhost:8080/api/transaction \
    -H "Content-Type: application/json" \
    -d '{
        "from": "0xf780fbF9ff9952897425f302b362eDBb80447108",
        "to": "0x4Bf75C4A3ab623bC92596227F8e75f0d6E7015a5", 
        "amount": 10,
        "privateKeyHex": "6f389f57a5bca725595e26972d8db9023fa092fe60b5e332760de48ffadc3dac"
    }'
```

## Hệ thống chưa xử lý việc tự động tạo tài khoản lên chain khi lần đầu có giao dịch

