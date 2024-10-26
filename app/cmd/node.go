package main

import (
	"encoding/json"
	"log"
	"main/internal/blockchain"
	"main/internal/p2pnetwork"
	"os"
)

// Định nghĩa cấu trúc cho file cấu hình
type Config struct {
	Nodes         []string `json:"nodes"`
	PrivateKeyHex string   `json:"privateKeyHex"`
}

func main() {
	// Khởi tạo blockchain
	blockchain.Init()

	// Lấy khối genesis
	genesisBlock := blockchain.GetGenesisBlock()
	log.Printf("Khối Genesis: %+v\n", genesisBlock)

	// Đọc file cấu hình
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Không thể đọc file cấu hình: %v", err)
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Fatalf("Không thể phân tích file cấu hình: %v", err)
	}

	// Tạo cấu hình cho P2P network
	p2pConfig := &p2pnetwork.Config{
		PrivateKeyHex: config.PrivateKeyHex,
		Nodes:         config.Nodes,
		MaxPeers:      10,
		Name:          "MyP2PNode",
		ListenAddr:    ":30303",
	}

	// Khởi tạo P2P network
	p2pNet, err := p2pnetwork.NewP2PNetwork(p2pConfig)
	if err != nil {
		log.Fatalf("Không thể khởi tạo P2P network: %v", err)
	}

	// Khởi động P2P network
	if err := p2pNet.Start(); err != nil {
		log.Fatalf("Không thể khởi động P2P network: %v", err)
	}
	defer p2pNet.Stop()

	// Chạy vô hạn để giữ chương trình hoạt động
	select {}
}
