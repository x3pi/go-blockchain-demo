package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
)

type Transaction struct {
	ID        []byte  // Hash của transaction
	From      string  // Địa chỉ người gửi
	To        string  // Địa chỉ người nhận
	Amount    float64 // Số tiền giao dịch
	Timestamp int64   // Thời gian tạo transaction
	Signature string  // Chữ ký số của transaction
}

// NewTransaction tạo một transaction mới
func NewTransaction(from string, to string, amount float64, privateKeyHex string) (*Transaction, error) {
	tx := &Transaction{
		From:      from,
		To:        to,
		Amount:    amount,
		Timestamp: time.Now().Unix(),
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
			[]byte(strconv.FormatFloat(tx.Amount, 'f', -1, 64)),
			[]byte(strconv.FormatInt(tx.Timestamp, 10)),
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
	message := fmt.Sprintf("%s_%s_%f_%d", tx.From, tx.To, tx.Amount, tx.Timestamp)
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
	message := fmt.Sprintf("%s_%s_%f_%d", tx.From, tx.To, tx.Amount, tx.Timestamp)
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

	// So sánh public key
	matches := bytes.Equal(publicKeyBytes, sigPublicKey)
	return matches, nil
}

// SaveTransactionToTrie lưu transaction vào trie
func (bc *Blockchain) SaveTransactionToTrie(tx *Transaction) error {
	txJSON, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	key := tx.ID // Sử dụng transaction ID làm key
	bc.txTrie.Update(key, txJSON)

	root, nodes := bc.txTrie.Commit(false)
	trieDB := triedb.NewDatabase(rawdb.NewDatabase(bc.db), &triedb.Config{})
	if err := trieDB.Update(root, common.Hash{}, 0, trienode.NewWithNodeSet(nodes), nil); err != nil {
		return fmt.Errorf("lỗi khi cập nhật transaction trie: %v", err)
	}

	if err := bc.db.Put([]byte(TX_ROOT_HASH), root[:]); err != nil {
		return fmt.Errorf("lỗi khi lưu transaction root hash: %v", err)
	}

	newTrie, err := bc.RestoreTrieFromRootHash(root)
	if err != nil {
		return fmt.Errorf("lỗi khi khôi phục trie từ root hash: %v", err)
	}
	bc.txTrie = newTrie
	fmt.Printf("Khôi phục trie từ root hash thành công: %s\n", root)

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
