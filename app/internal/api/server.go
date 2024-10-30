package api

import (
	"encoding/json"
	"fmt"
	"log"
	"main/internal/blockchain"
	"net/http"
)

type Server struct {
	blockchain *blockchain.Blockchain
}

func NewServer() *Server {
	return &Server{
		blockchain: blockchain.GetInstance(),
	}
}

func (s *Server) Start(port string) error {
	// Đăng ký các routes
	http.HandleFunc("/api/account/", s.handleGetAccount)

	log.Printf("Máy chủ API đang khởi động trên cổng %s", port)
	return http.ListenAndServe(port, nil)
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Phương thức không được phép", http.StatusMethodNotAllowed)
		return
	}

	// Lấy địa chỉ từ đường dẫn URL
	address := r.URL.Path[len("/api/account/"):]
	if address == "" {
		http.Error(w, "Yêu cầu phải có địa chỉ", http.StatusBadRequest)
		return
	}

	// Lấy tài khoản từ blockchain
	account, err := s.blockchain.GetAccountFromTrie(address)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi khi lấy tài khoản: %v", err), http.StatusNotFound)
		return
	}

	// Thiết lập header cho response
	w.Header().Set("Content-Type", "application/json")

	// Ghi response
	json.NewEncoder(w).Encode(account)
}
