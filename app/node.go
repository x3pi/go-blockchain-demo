package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	// "os" // Removed unused import
	"time"

	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// Định nghĩa cấu trúc cho file cấu hình
type Config struct {
	Nodes         []string `json:"nodes"`
	PrivateKeyHex string   `json:"privateKeyHex"`
}

func main() {
	// Đọc file cấu hình
	configFile, err := os.ReadFile("config.json") // Updated to use os.ReadFile
	if err != nil {
		log.Fatalf("Không thể đọc file cấu hình: %v", err)
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Fatalf("Không thể phân tích file cấu hình: %v", err)
	}

	// Tạo private key cho node từ chuỗi hex đã cho
	privateKeyHex := config.PrivateKeyHex
	nodekey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("Không thể tạo khóa node từ hex: %v", err)
	}

	privateKeyBytes := crypto.FromECDSA(nodekey)
	fmt.Printf("Khóa riêng: %x\n", privateKeyBytes)

	publicKey := nodekey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("lỗi khi chuyển đổi khóa công khai thành ECDSA")
	}
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	publicKeyString := hex.EncodeToString(publicKeyBytes)
	fmt.Printf("Khóa công khai: %s\n", publicKeyString)
	// Thêm dòng này để kiểm tra độ dài
	fmt.Printf("Độ dài khóa công khai: %d bytes\n", len(publicKeyBytes))

	// Tạo danh sách các node từ file cấu hình
	var nodes []*enode.Node
	for _, nodeAddr := range config.Nodes {
		nodes = append(nodes, enode.MustParse(nodeAddr))
	}

	// Tạo cấu hình cho server p2p
	cfg := p2p.Config{
		PrivateKey: nodekey,
		MaxPeers:   10,
		Name:       "MyP2PNode",
		ListenAddr: ":30303",
	}

	// Tạo protocol ping pong
	proto := p2p.Protocol{
		Name:    "ping",
		Version: 1,
		Length:  1,
		Run:     runPing,
	}

	// Thêm protocol vào server
	cfg.Protocols = []p2p.Protocol{proto}

	// Tạo và khởi động server
	srv := &p2p.Server{Config: cfg}
	if err := srv.Start(); err != nil {
		log.Fatalf("Không thể khởi động server: %v", err)
	}
	defer srv.Stop()

	// Ping các node trong danh sách
	for _, node := range nodes {
		srv.AddPeer(node)
	}

	// Chạy vô hạn để giữ chương trình hoạt động
	select {}
}

func runPing(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	for {
		err := p2p.Send(rw, 0, "PING")
		if err != nil {
			return fmt.Errorf("lỗi khi gửi PING đến %v: %v", peer.ID(), err)
		}

		msg, err := rw.ReadMsg()
		if err != nil {
			return fmt.Errorf("lỗi khi đọc tin nhắn từ %v: %v", peer.ID(), err)
		}

		var ping string
		err = msg.Decode(&ping)
		if err != nil {
			return fmt.Errorf("lỗi khi giải mã tin nhắn từ %v: %v", peer.ID(), err)
		}

		fmt.Printf("Nhận %s từ %v\n", ping, peer.ID())
		time.Sleep(20 * time.Second)
	}
}
