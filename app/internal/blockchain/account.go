package blockchain

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
)

// Account đại diện cho một tài khoản trong blockchain
type Account struct {
	Address   string  // Địa chỉ công khai của tài khoản
	Balance   float64 // Số dư của tài khoản
	PublicKey string  // Khóa công khai dạng hex
	Nonce     uint64  // Số transaction đã thực hiện từ tài khoản này
}

// NewAccount tạo một tài khoản mới từ public key
func NewAccountFromPublicKey(publicKeyHex string) (*Account, error) {
	// Giải mã public key từ hex
	publicKeyBytes, err := hexutil.Decode(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("public key không hợp lệ: %v", err)
	}

	// Chuyển đổi sang public key ECDSA
	publicKey, err := crypto.UnmarshalPubkey(publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("lỗi chuyển đổi public key: %v", err)
	}

	// Tạo địa chỉ từ public key
	address := crypto.PubkeyToAddress(*publicKey).Hex()

	return &Account{
		Address:   address,
		Balance:   0,
		PublicKey: publicKeyHex,
		Nonce:     0,
	}, nil
}

// GetBalance trả về số dư hiện tại của tài khoản
func (a *Account) GetBalance() float64 {
	return a.Balance
}

// UpdateBalance cập nhật số dư của tài khoản
func (a *Account) UpdateBalance(amount float64) {
	a.Balance = amount
}

// IncrementNonce tăng số nonce của tài khoản
func (a *Account) IncrementNonce() {
	a.Nonce++
}

// VerifyTransaction xác minh một transaction có được ký bởi chủ tài khoản này không
func (a *Account) VerifyTransaction(tx *Transaction) (bool, error) {
	return VerifyTransactionSignature(tx, a.PublicKey)
}

// SaveAccountToTrie lưu account vào trie
func (bc *Blockchain) SaveAccountToTrie(account *Account) error {
	accountJSON, err := json.Marshal(account)
	if err != nil {
		return err
	}

	key := []byte(account.Address)
	bc.accTrie.Update(key, accountJSON)

	root, nodes := bc.accTrie.Commit(false)
	trieDB := triedb.NewDatabase(rawdb.NewDatabase(bc.db), &triedb.Config{})
	if err := trieDB.Update(root, common.Hash{}, 0, trienode.NewWithNodeSet(nodes), nil); err != nil {
		return fmt.Errorf("lỗi khi cập nhật account trie: %v", err)
	}

	if err := bc.db.Put([]byte(ACC_ROOT_HASH), root[:]); err != nil {
		return fmt.Errorf("lỗi khi lưu account root hash: %v", err)
	}

	// Thêm bước khôi phục trie từ root hash mới
	newTrie, err := bc.RestoreTrieFromRootHash(root)
	if err != nil {
		return fmt.Errorf("lỗi khi khôi phục trie từ root hash: %v", err)
	}
	bc.accTrie = newTrie
	fmt.Printf("Khôi phục account trie từ root hash thành công: %s\n", root)

	return nil
}

// GetAccountFromTrie lấy account từ trie
func (bc *Blockchain) GetAccountFromTrie(address string) (*Account, error) {
	value, err := bc.accTrie.Get([]byte(address))
	if err != nil {
		return nil, fmt.Errorf("lỗi khi lấy account: %v", err)
	}
	if value == nil {
		return nil, fmt.Errorf("không tìm thấy account với địa chỉ: %s", address)
	}

	var account Account
	if err = json.Unmarshal(value, &account); err != nil {
		return nil, fmt.Errorf("lỗi khi giải mã account: %v", err)
	}
	return &account, nil
}
