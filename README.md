

# Blockchain P2P

## 1. Cấu trúc dữ liệu Trie

Project sử dụng Modified Merkle Patricia Trie (MPT) từ thư viện go-ethereum để lưu trữ các block. Trie có các đặc điểm:

- Lưu trữ cặp key-value cho các block

Tham khảo code triển khai tại:

```239:242:app/internal/blockchain/chain.go
// Tạo một trie mới
func CreateNewTrie(db *triedb.Database) (*trie.Trie, error) {
	return trie.New(trie.TrieID(common.Hash{}), db)
}
```


## 2. LevelDB

Sử dụng LevelDB làm cơ sở dữ liệu key-value để:

- Lưu trữ các block có thể truy vấn qua trie của go-ethereum
- Lưu trữ root hash của Trie
- Lưu trữ thông tin về block hiện tại và block cuối cùng

Các key chính được định nghĩa:

```23:28:app/internal/blockchain/chain.go
// Định nghĩa các hằng số cho khóa cơ sở dữ liệu
const (
	ROOT_HASH     = "ROOT_HASH"
	CURRENT_BLOCK = "CURRENT_BLOCK"
	LAST_BLOCK    = "LAST_BLOCK"
)
```


## 3. P2P Network

Sử dụng thư viện p2p của go-ethereum để xây dựng mạng ngang hàng:

- Mỗi node có private key và public key riêng
- Các node kết nối với nhau thông qua enode URL
- Giao thức trao đổi block giữa các node qua đề xuất block và yêu cầu block
- Đồng bộ hóa trạng thái blockchain

Cấu trúc P2P Network:

```16:21:app/internal/p2pnetwork/p2pnetwork.go
type P2PNetwork struct {
	Config     *Config
	Server     *p2p.Server
	PrivateKey *ecdsa.PrivateKey
	blockchain *blockchain.Blockchain
}
```


## 4. Chữ ký ECDSA

Sử dụng ECDSA (Elliptic Curve Digital Signature Algorithm) để:

- Dùng private key là privateKeyHex khi khởi tạo node p2p luôn
- Tạo chữ ký số cho các block
- Xác thực chữ ký của block từ các node khác
- Đảm bảo tính xác thực của block

Các hàm chính về chữ ký:

```454:506:app/internal/blockchain/chain.go
func SignMerkleRoot(privateKeyHex string, message string) (string, error) {
	// Chuyển đổi khóa riêng hex thành khóa riêng ECDSA
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("khóa riêng không hợp lệ: %v", err)
	}
	publicKey := crypto.FromECDSAPub(&privateKey.PublicKey)
	fmt.Printf("Khóa công khai: %s\n", hexutil.Encode(publicKey))
	// Băm thông điệp
	messageHash := crypto.Keccak256Hash([]byte(message))

	// Ký băm
	signature, err := crypto.Sign(messageHash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("lỗi khi ký thông điệp: %v", err)
	}

	// Chuyển đổi chữ ký thành chuỗi hex
	return hexutil.Encode(signature), nil
}

// VerifySignature xác minh xem chữ ký có hợp lệ không
func VerifySignature(publicKeyHex string, message string, signatureHex string) (bool, error) {
	// Giải mã chữ ký từ hex
	signature, err := hexutil.Decode(signatureHex)
	if err != nil {
		return false, fmt.Errorf("chữ ký hex không hợp lệ: %v", err)
	}

	// Băm thông điệp
	messageHash := crypto.Keccak256Hash([]byte(message))

	// Giải mã khóa công khai từ hex
	publicKeyBytes, err := hexutil.Decode(publicKeyHex)
	if err != nil {
		return false, fmt.Errorf("khóa công khai hex không hợp lệ: %v", err)
	}

	// Chuyển đổi bytes thành khóa công khai
	publicKey, err := crypto.UnmarshalPubkey(publicKeyBytes)
	if err != nil {
		return false, fmt.Errorf("khóa công khai không hợp lệ: %v", err)
	}

	// Xác minh chữ ký
	sigPublicKey, err := crypto.Ecrecover(messageHash.Bytes(), signature)
	if err != nil {
		return false, fmt.Errorf("lỗi khi phục hồi khóa công khai: %v", err)
	}

	matches := bytes.Equal(crypto.FromECDSAPub(publicKey), sigPublicKey)
	return matches, nil
}
```


## 5. Luồng hoạt động

1. Khởi tạo:
- Đọc cấu hình từ file config.json
- Khởi tạo LevelDB và Trie
- Tạo kết nối P2P với các node khác

2. Xử lý block:
- Đề xuất block mới theo chu kỳ
- Ký block bằng private key
- Broadcast block đến các node khác
- Xác thực và lưu trữ block nhận được

3. Đồng bộ hóa:
- Kiểm tra block hiện tại và block cuối
- Yêu cầu các block còn thiếu từ các node khác
- Cập nhật trạng thái local

## 6. Cấu hình Node

Mỗi node có file cấu hình riêng chứa:
- Private key của node
- Public key và URL của các node khác
- Chỉ số node trong mạng

Ví dụ cấu hình node:

```1:16:app/config/config1.json
{
	"privateKeyHex": "fdf0edfd86751d13e9ecbbf0c3a1b197cbccfa1ad5758aaeb22a54dc967fda9f",
	"index": 1,
	"nodes": [
		{
			"publicKey": "8f1273da4d7c042caa74c4fe50443831875128a8ff7817c40f1211cdf6e65e63e5ce5139da1983946cb15a054d951559523d7121ae9d0314f5e187cc757b36e2",
			"url": "host.docker.internal:30304",
			"index": 2
		},
		{
			"publicKey": "d53415bd1e6941e971c2eb1f9a4e964dd8cb043a5865f023f004915aef78d462d61072e9448ea334070cc26460c471a6c6ca9bc82461c02ddea5ca2bf7c170f6",
			"url": "host.docker.internal:30305",
			"index": 3
		}
	]
}
```

# Chú ý
- Chỉ sử dụng để demo và học tập
- Không đảm bảo toàn vẹn dữ liệu của block vì chỉ chỉ key lưu trữ đánh dấu theo index và không băm dữ liệu.
- Không có ràng buộc nào 2/3 node cùng chạy nên 1 node vẫn chạy được. Các node được đề xuất block tuần tự theo file config.
- Chưa xử lý node offline có quyền đề xuất lại khối chưa đề xuất.
- Chứa xử lý trường hợp nếu 1 node cố tình gửi block giả mạo.
- Rất nhiều vấn đề nữa nhưng demo mục đích sử dụng công cụ cơ bản để có thể tạo nên một blockchain và làm quen với các module của go-ethereum.



# Hướng dẫn chạy Docker

## Yêu cầu
- Docker
- Docker Compose

## Các bước thực hiện

### 1. Cấu trúc thư mục
```
app/
├── cmd/
│   └── node/
├── config/
│   ├── config1.json
│   ├── config2.json 
│   └── config3.json
├── internal/
├── Dockerfile
└── docker-compose.yml
```

### 2. Kiểm tra Dockerfile
Dockerfile đã được cấu hình đúng:

```1:17:app/Dockerfile
# Dockerfile
FROM golang:1.23.2

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
```


### 3. Kiểm tra docker-compose.yml
File docker-compose.yml đã cấu hình 3 node:

```1:24:app/docker-compose.yml
# docker-compose.yml
version: '3.8'

services:
  node1:
    build: .
    volumes:
      - ./config/config1.json:/app/config.json  # Gắn file config.json vào container
    ports:
      - "30303:30303"  # Mở cổng 30303

  node2:  # Node mới
    build: .
    volumes:
      - ./config/config2.json:/app/config.json  # Gắn file config.json vào container
    ports:
      - "30304:30303"  # Mở cổng 30304

  node3:  # Node mới
    build: .
    volumes:
      - ./config/config3.json:/app/config.json  # Gắn file config.json vào container
    ports:
      - "30305:30303"  # Mở cổng 30305
```


### 4. Các bước chạy

1. Di chuyển vào thư mục chứa docker-compose.yml:
```bash
cd app
```

2. Build các image:
```bash
docker-compose build
```

3. Khởi chạy các container:
```bash
docker-compose up
```

Hoặc chạy ngầm:
```bash
docker-compose up -d
```

4. Kiểm tra các container đang chạy:
```bash
docker-compose ps
```

5. Xem logs của từng node:
```bash
# Xem log node 1
docker-compose logs node1

# Xem log node 2  
docker-compose logs node2

# Xem log node 3
docker-compose logs node3
```

6. Dừng và xóa các container:
```bash
docker-compose down
```

### 5. Cấu trúc mạng
- Node 1: Port 30303
- Node 2: Port 30304  
- Node 3: Port 30305

Các node sẽ tự động kết nối với nhau thông qua cấu hình trong file config*.json

### 6. Lưu ý
- Đảm bảo các port 30303, 30304, 30305 không bị sử dụng bởi ứng dụng khác
- Kiểm tra logs để xem trạng thái kết nối giữa các node
- Chưa có cơ chế để tăng hoặc giảm node