package api

import (
	"encoding/json"
	"fmt"
	"log"
	"main/internal/blockchain"
	"net/http"

	"github.com/ethereum/go-ethereum/common/hexutil"
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
	http.HandleFunc("/api/transaction", s.handleTransaction)
	http.HandleFunc("/api/transaction/", s.handleGetTransaction)

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

	// Tạo response structure
	response := struct {
		Success bool `json:"success"`
		Data    struct {
			Address   string `json:"address"`
			Balance   uint64 `json:"balance"`
			PublicKey string `json:"publicKey"`
			Nonce     uint64 `json:"nonce"`
		} `json:"data"`
	}{
		Success: true,
		Data: struct {
			Address   string `json:"address"`
			Balance   uint64 `json:"balance"`
			PublicKey string `json:"publicKey"`
			Nonce     uint64 `json:"nonce"`
		}{
			Address:   account.Address,
			Balance:   account.Balance,
			PublicKey: account.PublicKey,
			Nonce:     account.Nonce,
		},
	}

	// Thiết lập header cho response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Ghi response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi khi mã hóa response: %v", err), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Phương thức không được phép", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		From          string  `json:"from"`
		To            string  `json:"to"`
		Amount        float64 `json:"amount"`
		PrivateKeyHex string  `json:"privateKeyHex"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi khi đọc dữ liệu: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.From == "" || req.To == "" || req.Amount <= 0 || req.PrivateKeyHex == "" {
		http.Error(w, "Thiếu thông tin giao dịch", http.StatusBadRequest)
		return
	}

	// Create and add transaction
	tx, err := s.blockchain.CreateAndAddTransaction(req.From, req.To, req.Amount, req.PrivateKeyHex)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi khi tạo giao dịch: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// Write response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"transaction": map[string]interface{}{
			"id":        hexutil.Encode(tx.ID),
			"from":      tx.From,
			"to":        tx.To,
			"amount":    tx.Amount,
			"timestamp": tx.Timestamp,
		},
	})
}

func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Phương thức không được phép", http.StatusMethodNotAllowed)
		return
	}

	// Get transaction ID from URL path
	txID := r.URL.Path[len("/api/transaction/"):]
	if txID == "" {
		http.Error(w, "Yêu cầu phải có ID giao dịch", http.StatusBadRequest)
		return
	}

	// Get transaction from blockchain
	tx, err := s.blockchain.GetTransactionByID(txID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi khi lấy giao dịch: %v", err), http.StatusNotFound)
		return
	}

	// Create response structure
	response := struct {
		Success bool `json:"success"`
		Data    struct {
			ID        string `json:"id"`
			From      string `json:"from"`
			To        string `json:"to"`
			Amount    uint64 `json:"amount"`
			Timestamp uint64 `json:"timestamp"`
		} `json:"data"`
	}{
		Success: true,
		Data: struct {
			ID        string `json:"id"`
			From      string `json:"from"`
			To        string `json:"to"`
			Amount    uint64 `json:"amount"`
			Timestamp uint64 `json:"timestamp"`
		}{
			ID:        hexutil.Encode(tx.ID),
			From:      tx.From,
			To:        tx.To,
			Amount:    tx.Amount,
			Timestamp: tx.Timestamp,
		},
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Write response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Lỗi khi mã hóa response: %v", err), http.StatusInternalServerError)
		return
	}
}
