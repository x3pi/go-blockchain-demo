package main

import (
	"fmt"
	"log"
	"main/internal/api"
	"main/internal/blockchain"
	"main/internal/p2pnetwork"
)

func main() {
	// Khởi tạo blockchain
	bc := blockchain.GetInstance()
	// Lưu toàn bộ cấu hình vào blockchain
	if err := bc.Init(); err != nil {
		log.Fatalf("Không thể khởi tạo blockchain: %v", err)
	}

	// Initialize and start HTTP API server in a goroutine
	apiServer := api.NewServer()
	go func() {
		if err := apiServer.Start(":8080"); err != nil {
			log.Fatalf("Không thể khởi động API server: %v", err)
		}
	}()

	// Truy cập cấu hình từ blockchain
	privateKeyHex := bc.Config.PrivateKeyHex // Truy cập trường PrivateKeyHex từ config

	// Tạo mảng nodes cho P2P network
	var enodeNodes []string
	for _, node := range bc.Config.Nodes {
		if node.Index != bc.Config.Index {
			// Format enode URL with proper scheme and port
			enodeURL := fmt.Sprintf("enode://%s@%s", node.PublicKey, node.URL)
			log.Printf("Adding enode URL: %s", enodeURL)
			enodeNodes = append(enodeNodes, enodeURL)
		}
	}

	// Tạo cấu hình cho P2P network
	p2pConfig := &p2pnetwork.Config{
		PrivateKeyHex: privateKeyHex,
		Nodes:         enodeNodes,
		MaxPeers:      10,
		Name:          fmt.Sprintf("Node%d", bc.Config.Index),
		ListenAddr:    ":30303",
	}

	// Khởi tạo P2P network
	p2pNet, err := p2pnetwork.NewP2PNetwork(p2pConfig)
	if err != nil {
		log.Fatalf("Không thể khởi tạo P2P network: %v", err)
	}

	// Thiết lập P2PNetwork cho blockchain
	bc.SetP2PNetwork(p2pNet)

	// Kiểm tra P2P network đã được thiết lập
	if bc.P2PNetwork == nil {
		log.Fatal("P2P network chưa được thiết lập đúng")
	}
	log.Printf("P2P network đã được thiết lập thành công")

	// Khởi động P2P network
	if err := p2pNet.Start(); err != nil {
		log.Fatalf("Không thể khởi động P2P network: %v", err)
	}
	log.Printf("P2P network đã khởi động thành công với địa chỉ lắng nghe %s", p2pConfig.ListenAddr)
	defer p2pNet.Stop()

	// Chạy vô hạn để giữ chương trình hoạt động
	select {}
}
