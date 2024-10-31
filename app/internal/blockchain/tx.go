package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
)

type Transaction struct {
	ID        []byte // Hash của transaction
	From      string // Địa chỉ người gửi
	To        string // Địa chỉ người nhận
	Amount    uint64 // Số tiền giao dịch
	Timestamp uint64 // Thời gian tạo transaction
	Signature string // Chữ ký số của transaction
}

// NewTransaction tạo một transaction mới
func NewTransaction(from string, to string, amount float64, privateKeyHex string) (*Transaction, error) {
	tx := &Transaction{
		From:      from,
		To:        to,
		Amount:    uint64(amount),
		Timestamp: uint64(time.Now().Unix()),
	}
	// Tạo ID bằng cách hash các thông tin của transaction
	tx.ID = tx.Hash()

	// Tạo chữ ký số cho transaction
	signature, err := SignTransaction(tx, privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi ký transaction: %v", err)
	}
	tx.Signature = signature

	return tx, nil
}

// Hash tạo hash cho transaction
func (tx *Transaction) Hash() []byte {
	data := bytes.Join(
		[][]byte{
			[]byte(tx.From),
			[]byte(tx.To),
			[]byte(strconv.FormatUint(tx.Amount, 10)),
			[]byte(strconv.FormatUint(tx.Timestamp, 10)),
		},
		[]byte{},
	)
	hash := sha256.Sum256(data)
	return hash[:]
}

// SignTransaction ký một transaction bằng khóa riêng ECDSA
func SignTransaction(tx *Transaction, privateKeyHex string) (string, error) {
	// Chuyển đổi khóa riêng hex thành khóa ECDSA
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("khóa riêng không hợp lệ: %v", err)
	}

	// Tạo message hash từ transaction
	message := fmt.Sprintf("%s_%s_%d_%d", tx.From, tx.To, tx.Amount, tx.Timestamp)
	messageHash := crypto.Keccak256Hash([]byte(message))

	// Ký hash bằng khóa riêng
	signature, err := crypto.Sign(messageHash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("lỗi khi ký transaction: %v", err)
	}

	// Chuyển chữ ký thành hex string
	return hexutil.Encode(signature), nil
}

// VerifyTransactionSignature xác minh chữ ký của transaction
func VerifyTransactionSignature(tx *Transaction, publicKeyHex string) (bool, error) {
	// Tạo message hash từ transaction
	message := fmt.Sprintf("%s_%s_%d_%d", tx.From, tx.To, tx.Amount, tx.Timestamp)
	messageHash := crypto.Keccak256Hash([]byte(message))

	// Giải mã chữ ký từ hex
	signature, err := hexutil.Decode(tx.Signature)
	if err != nil {
		return false, fmt.Errorf("chữ ký không hợp lệ: %v", err)
	}

	// Giải mã public key từ hex
	publicKeyBytes, err := hexutil.Decode(publicKeyHex)
	if err != nil {
		return false, fmt.Errorf("khóa công khai không hợp lệ: %v", err)
	}

	// Khôi phục public key từ chữ ký
	sigPublicKey, err := crypto.Ecrecover(messageHash.Bytes(), signature)
	if err != nil {
		return false, fmt.Errorf("lỗi khi phục hồi khóa công khai: %v", err)
	}

	// In ra các giá trị để debug
	fmt.Printf("Public Key (hex):     %x\n", publicKeyBytes)
	fmt.Printf("Recovered Public Key: %x\n", sigPublicKey)

	// So sánh public key

	matches := bytes.Equal(publicKeyBytes, sigPublicKey)
	return matches, nil
}

// SaveTransactionToTrie lưu transaction vào trie
func (bc *Blockchain) SaveTransactionToTrie(tx *Transaction) error {

	fmt.Printf("Bắt đầu lưu giao dịch tx: %+v\n", tx)

	txJSON, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("lỗi marshal transaction: %v", err)
	}

	key := tx.ID
	bc.txTrie.Update(key, txJSON)

	fmt.Printf("Hoàn thành updte giao dịch tx: %+v\n", tx)

	root, nodes := bc.txTrie.Commit(false)
	trieDB := triedb.NewDatabase(rawdb.NewDatabase(bc.db), &triedb.Config{})

	// Log thông tin debug
	fmt.Printf("Root hash: %x\n", root)

	if err := trieDB.Update(root, common.Hash{}, 0, trienode.NewWithNodeSet(nodes), nil); err != nil {
		return fmt.Errorf("lỗi khi cập nhật trieDB: %v", err)
	}

	if err := trieDB.Commit(root, false); err != nil {
		return fmt.Errorf("lỗi khi commit trie database: %v", err)
	}

	fmt.Printf("Hoàn thành cập nhật trieDB cho giao dịch: %+v\n", tx)
	fmt.Printf("Bắt đầu lưu root hash cho giao dịch: %+v\n", root)

	if err := bc.db.Put([]byte(TX_ROOT_HASH), root[:]); err != nil {
		return fmt.Errorf("lỗi khi lưu root hash: %v", err)
	}

	// // Kiểm tra root hash đã được lưu
	// savedRoot, err := bc.db.Get([]byte(TX_ROOT_HASH))
	// if err != nil {
	// 	return fmt.Errorf("lỗi khi đọc lại root hash: %v", err)
	// }
	// fmt.Printf("Saved root hash: %x\n", savedRoot)

	// fmt.Printf("Bắt đầu khôi phục trie giao dịch với root hash: %x\n", root)

	// Thử khôi phục trie
	newTrie, err := bc.RestoreTrieFromRootHash(root)
	if err != nil {
		return fmt.Errorf("lỗi khôi phục trie (root: %x): %v", root, err)
	}

	bc.txTrie = newTrie
	fmt.Printf("Khôi phục trie thành công với root hash: %x\n", root)

	return nil
}

// GetTransactionFromTrie lấy transaction từ trie
func (bc *Blockchain) GetTransactionFromTrie(txID []byte) (*Transaction, error) {
	value, err := bc.txTrie.Get(txID)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi lấy transaction: %v", err)
	}
	if value == nil {
		return nil, fmt.Errorf("không tìm thấy transaction với ID: %x", txID)
	}

	var tx Transaction
	if err = json.Unmarshal(value, &tx); err != nil {
		return nil, fmt.Errorf("lỗi khi giải mã transaction: %v", err)
	}
	return &tx, nil
}

// Mempool cấu trúc để lưu trữ các giao dịch đang chờ x lý
type Mempool struct {
	transactions map[string]*Transaction // ánh x hash giao dịch tới giao dịch
	mutex        sync.RWMutex
}

// NewMempool tạo một thể hiện mempool mới
func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*Transaction),
	}
}

// AddTransaction thêm một giao dịch vào mempool
func (mp *Mempool) AddTransaction(tx *Transaction) error {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	// Convert transaction ID to string for map key
	txID := hexutil.Encode(tx.ID)

	// Check if transaction already exists
	if _, exists := mp.transactions[txID]; exists {
		return fmt.Errorf("giao dịch đã tồn tại trong mempool: %s", txID)
	}

	// Add to mempool
	mp.transactions[txID] = tx
	return nil
}

// GetTransaction lấy một giao dịch từ mempool bằng ID
func (mp *Mempool) GetTransaction(txID string) (*Transaction, error) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	tx, exists := mp.transactions[txID]
	if !exists {
		return nil, fmt.Errorf("không tìm thấy giao dịch trong mempool: %s", txID)
	}
	return tx, nil
}

// RemoveTransaction xóa một giao dịch khỏi mempool
func (mp *Mempool) RemoveTransaction(txID string) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	delete(mp.transactions, txID)
}

// GetAllTransactions trả về tất cả các giao dịch trong mempool
func (mp *Mempool) GetAllTransactions() []*Transaction {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	txs := make([]*Transaction, 0, len(mp.transactions))
	for _, tx := range mp.transactions {
		txs = append(txs, tx)
	}
	return txs
}

// GetSize trả về số lượng giao dịch trong mempool
func (mp *Mempool) GetSize() int {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	return len(mp.transactions)
}

// CreateAndAddTransaction tạo và thêm một giao dịch mới vào mempool
func (bc *Blockchain) CreateAndAddTransaction(from, to string, amount float64, privateKeyHex string) (*Transaction, error) {
	// Tạo transaction mới
	tx, err := NewTransaction(from, to, amount, privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi tạo transaction: %v", err)
	}

	// Lấy account từ trie account
	fromAccount, err := bc.GetAccountFromTrie(from)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi lấy thông tin tài khoản người gửi: %v", err)
	}

	// // Kiểm tra số dư có đủ không
	// if fromAccount.Balance < uint64(amount) {
	// 	return nil, fmt.Errorf("số dư không đủ")
	// }

	// Verify signature sử dụng public key từ account
	isValid, err := VerifyTransactionSignature(tx, fromAccount.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi xác thực chữ ký: %v", err)
	}
	if !isValid {
		return nil, fmt.Errorf("chữ ký transaction không hợp lệ")
	}

	// Thêm vào mempool
	if err := bc.Mempool.AddTransaction(tx); err != nil {
		return nil, fmt.Errorf("lỗi khi thêm vào mempool: %v", err)
	}

	bc.Mempool.PrintMempool()

	// // Lưu transaction vào trie
	// if err := bc.SaveTransactionToTrie(tx); err != nil {
	// 	// Nếu lưu vào trie thất bại, xóa khỏi mempool
	// 	bc.Mempool.RemoveTransaction(hexutil.Encode(tx.ID))
	// 	return nil, fmt.Errorf("lỗi khi lưu transaction vào trie: %v", err)
	// }

	// Debug print for P2PNetwork
	fmt.Printf("P2PNetwork status: %v\n", bc.P2PNetwork != nil)

	if bc.P2PNetwork != nil {
		if err := bc.P2PNetwork.BroadcastTransaction(tx); err != nil {
			// Log lỗi nhưng không return vì transaction đã được thêm vào mempool
			log.Printf("Cảnh báo: Lỗi khi broadcast giao dịch: %v", err)
		} else {
			log.Printf("Đã broadcast giao dịch thành công: %x", tx.ID)
		}
	}

	return tx, nil
}

// GetMempoolTransactions trả về tất cả các giao dịch trong mempool
func (bc *Blockchain) GetMempoolTransactions() []*Transaction {
	return bc.Mempool.GetAllTransactions()
}

// GetMempoolSize trả về số lượng giao dịch trong mempool
func (bc *Blockchain) GetMempoolSize() int {
	return bc.Mempool.GetSize()
}

// GetTransactionByID trả về giao dịch theo ID
func (bc *Blockchain) GetTransactionByID(txID string) (*Transaction, error) {
	// Try to find the transaction in the local mempool first
	tx, err := bc.Mempool.GetTransaction(txID)
	if err == nil {
		return tx, nil
	}

	// If not found in mempool, try to get it from the trie
	txIDBytes, err := hexutil.Decode(txID)
	if err != nil {
		return nil, fmt.Errorf("ID giao dịch không hợp lệ: %v", err)
	}

	tx, err = bc.GetTransactionFromTrie(txIDBytes)
	if err == nil {
		return tx, nil
	}

	// // If not found locally, iterate through the nodes and request the transaction
	// nodes := bc.Config.Nodes // Assuming you have a Config field in Blockchain that holds the node configurations
	// for _, node := range nodes {
	// 	// Check if the node index is different from the current blockchain index
	// 	if node.Index != bc.Config.Index {
	// 		apiUrl := fmt.Sprintf("http://%s/api/transaction/%s", node.ApiUrl, txID)
	// 		resp, err := http.Get(apiUrl)
	// 		if err != nil {
	// 			log.Printf("Lỗi khi gọi API từ node %s: %v", node.ApiUrl, err)
	// 			continue // Try the next node
	// 		}
	// 		defer resp.Body.Close()

	// 		if resp.StatusCode == http.StatusOK {
	// 			var transaction Transaction
	// 			if err := json.NewDecoder(resp.Body).Decode(&transaction); err != nil {
	// 				log.Printf("Lỗi khi giải mã transaction từ node %s: %v", node.ApiUrl, err)
	// 				continue // Try the next node
	// 			}
	// 			return &transaction, nil
	// 		}
	// 	}
	// }

	// Nếu không tìm thấy locally, thử qua P2P network
	if bc.P2PNetwork != nil {
		// Tạo channel để nhận response
		responseChan := make(chan *Transaction)
		errorChan := make(chan error)
		timeout := time.After(10 * time.Second)

		go func() {
			// Gọi method để request transaction từ P2P network
			tx, err := bc.P2PNetwork.RequestTransaction(txID)
			if err != nil {
				errorChan <- err
				return
			}
			responseChan <- tx
		}()

		// Chờ response hoặc timeout
		select {
		case tx := <-responseChan:
			// Lưu transaction vào local để sử dụng sau này
			// if err := bc.SaveTransactionToTrie(tx); err != nil {
			// 	log.Printf("Cảnh báo: Không thể lưu transaction từ P2P vào trie: %v", err)
			// }
			return tx, nil
		case err := <-errorChan:
			return nil, fmt.Errorf("lỗi khi lấy transaction từ P2P network: %v", err)
		case <-timeout:
			return nil, fmt.Errorf("timeout khi chờ phản hồi từ P2P network")
		}
	}

	return nil, fmt.Errorf("không tìm thấy giao dịch với ID: %s", txID)
}

// PrintMempool in ra nội dung của mempool
func (mp *Mempool) PrintMempool() {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	fmt.Println("=== Mempool Contents ===")
	if len(mp.transactions) == 0 {
		fmt.Println("Mempool is empty")
		return
	}

	for txID, tx := range mp.transactions {
		fmt.Printf("\nTransaction ID: %s\n", txID)
		fmt.Printf("From: %s\n", tx.From)
		fmt.Printf("To: %s\n", tx.To)
		fmt.Printf("Amount: %d\n", tx.Amount)
		fmt.Printf("Timestamp: %d\n", tx.Timestamp)
		fmt.Printf("Signature: %s\n", tx.Signature[:64]+"...") // Chỉ hiển thị một phần chữ ký
		fmt.Println("------------------------")
	}
}

// ProcessMempoolTransactions xử lý tất cả các giao dịch trong mempool
func (bc *Blockchain) ProcessMempoolTransactions() ([]string, error) {
	// Lấy tất cả giao dịch từ mempool
	transactions := bc.Mempool.GetAllTransactions()
	successfulTxs := make([]string, 0)
	fmt.Printf("Processing %d transactions\n", len(transactions))

	for _, tx := range transactions {
		txID := hexutil.Encode(tx.ID)
		fmt.Printf("Processing transaction -----------: %s\n", txID)
		// Lấy thông tin tài khoản người gửi và người nhận
		fromAcc, err := bc.GetAccountFromTrie(tx.From)
		if err != nil {
			return successfulTxs, fmt.Errorf("lỗi khi lấy tài khoản người gửi: %v: %v", fromAcc, err)
		}

		toAcc, err := bc.GetAccountFromTrie(tx.To)
		if err != nil {
			return successfulTxs, fmt.Errorf("lỗi khi lấy tài khoản người nhận: %v: %v", toAcc, err)
		}

		// Kiểm tra số dư
		if fromAcc.Balance < tx.Amount {
			bc.Mempool.RemoveTransaction(txID)
			continue // Bỏ qua giao dịch này nếu không đủ số dư
		}

		// Cập nhật số dư
		fromAcc.Balance -= tx.Amount
		toAcc.Balance += tx.Amount

		// Tăng nonce của tài khoản người gửi
		fromAcc.IncrementNonce()

		fmt.Printf("fromAcc: %+v\n", fromAcc)
		fmt.Printf("toAcc: %+v\n", toAcc)
		// Lưu các thay đổi vào trie
		if err := bc.SaveAccountToTrie(fromAcc); err != nil {
			return successfulTxs, fmt.Errorf("lỗi khi cập nhật tài khoản người gửi: %v", err)
		}

		if err := bc.SaveAccountToTrie(toAcc); err != nil {
			return successfulTxs, fmt.Errorf("lỗi khi cập nhật tài khoản người nhận: %v", err)
		}

		// Lưu giao dịch vào trie
		if err := bc.SaveTransactionToTrie(tx); err != nil {
			return successfulTxs, fmt.Errorf("lỗi khi lưu giao dịch: %v", err)
		}

		// Thêm transaction ID vào danh sách thành công
		successfulTxs = append(successfulTxs, txID)

		// Xóa giao dịch khỏi mempool
		bc.Mempool.RemoveTransaction(txID)
		fmt.Printf("Hoàn tất giao dịch-------------------------------------------------------------: %s\n", txID)
	}

	return successfulTxs, nil
}

// ProcessTransactionsByIDs xử lý danh sách giao dịch từ các ID
func (bc *Blockchain) ProcessTransactionsByIDs(txIDs []string) error {
	for _, txID := range txIDs {
		// Lấy giao dịch từ trie hoặc mempool
		tx, err := bc.GetTransactionByID(txID)
		if err != nil {
			return fmt.Errorf("lỗi khi lấy giao dịch %s: %v", txID, err)
		}

		// Lấy thông tin tài khoản người gửi và người nhận
		fromAcc, err := bc.GetAccountFromTrie(tx.From)
		if err != nil {
			return fmt.Errorf("lỗi khi lấy tài khoản người gửi cho giao dịch %v: %v", fromAcc, err)
		}

		toAcc, err := bc.GetAccountFromTrie(tx.To)
		if err != nil {
			return fmt.Errorf("lỗi khi lấy tài khoản người nhận cho giao dịch %v: %v", toAcc, err)
		}

		// Kiểm tra số dư
		if fromAcc.Balance < tx.Amount {
			continue // Bỏ qua giao dịch này nếu không đủ số dư
		}

		// Cập nhật số dư
		fromAcc.Balance -= tx.Amount
		toAcc.Balance += tx.Amount

		// Tăng nonce của tài khoản người gửi
		fromAcc.IncrementNonce()

		// Lưu các thay đổi vào trie
		if err := bc.SaveAccountToTrie(fromAcc); err != nil {
			return fmt.Errorf("lỗi khi cập nhật tài khoản người gửi cho giao dịch %s: %v", txID, err)
		}

		if err := bc.SaveAccountToTrie(toAcc); err != nil {
			return fmt.Errorf("lỗi khi cập nhật tài khoản người nhận cho giao dịch %s: %v", txID, err)
		}

		// Lưu giao dịch vào trie
		if err := bc.SaveTransactionToTrie(tx); err != nil {
			return fmt.Errorf("lỗi khi lưu giao dịch %s: %v", txID, err)
		}

		// Xóa giao dịch khỏi mempool nếu có
	}

	return nil
}
