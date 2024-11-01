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
	Address   string // Địa chỉ công khai của tài khoản
	Balance   uint64 // Số dư của tài khoản
	PublicKey string // Khóa công khai dạng hex
	Nonce     uint64 // Số transaction đã thực hiện từ tài khoản này
}

// NewAccount tạo một tài khoản mới từ public key
func NewAccountFromPublicKey(publicKeyHex string, balance uint64) (*Account, error) {
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
		Balance:   balance,
		PublicKey: publicKeyHex,
		Nonce:     0,
	}, nil
}

// GetBalance trả về số dư hiện tại của tài khoản
func (a *Account) GetBalance() uint64 {
	return a.Balance
}

// UpdateBalance cập nhật số dư của tài khoản
func (a *Account) UpdateBalance(amount uint64) {
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

	// Lưu các node vào database
	if err := trieDB.Update(root, common.Hash{}, 0, trienode.NewWithNodeSet(nodes), nil); err != nil {
		return fmt.Errorf("lỗi khi cập nhật account trie: %v", err)
	}

	// Commit để đảm bảo tất cả các node được lưu
	if err := trieDB.Commit(root, false); err != nil {
		return fmt.Errorf("lỗi khi commit trie database: %v", err)
	}

	if err := bc.db.Put([]byte(ACC_ROOT_HASH), root[:]); err != nil {
		return fmt.Errorf("lỗi khi lưu account root hash: %v", err)
	}

	// Khôi phục lại trie để có thể tiếp tục đọc
	newTrie, err := bc.RestoreTrieFromRootHash(root)
	if err != nil {
		return fmt.Errorf("lỗi khi khôi phục trie từ root hash: %v", err)
	}
	bc.accTrie = newTrie

	fmt.Printf("Save account thành công : %+v\n", account)
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

// InitializeTestAccounts khởi tạo và lưu hai account test vào trie
func (bc *Blockchain) InitializeTestAccounts() error {
	// Thêm "0x" prefix nếu chưa có
	pubKey1 := "0x048f1273da4d7c042caa74c4fe50443831875128a8ff7817c40f1211cdf6e65e63e5ce5139da1983946cb15a054d951559523d7121ae9d0314f5e187cc757b36e2"
	pubKey2 := "0x04127bae1dc0022eabf3fc16447d501fa45906d5127d116de654b6e93b0606ee9430552b8f458d905459396d581b9201b7745edf1d2f47a84aed5e063cc196942b"
	pubKey3 := "0x04d53415bd1e6941e971c2eb1f9a4e964dd8cb043a5865f023f004915aef78d462d61072e9448ea334070cc26460c471a6c6ca9bc82461c02ddea5ca2bf7c170f6"

	// Khởi tạo account thứ nhất với số dư ban đầu
	acc1, err := NewAccountFromPublicKey(pubKey1, 1000)
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo account 1: %v", err)
	}
	// tạo ra tài khoản 0xf780fbF9ff9952897425f302b362eDBb80447108

	// Kiểm tra account 1 đã tồn tại chưa
	existingAcc1, err := bc.GetAccountFromTrie(acc1.Address)
	if err == nil && existingAcc1 != nil {
		fmt.Printf("Account 1 đã tồn tại: %s\n", acc1.Address)
	} else {
		// Lưu account 1 vào trie nếu chưa tồn tại
		if err := bc.SaveAccountToTrie(acc1); err != nil {
			return fmt.Errorf("lỗi lưu account 1: %v", err)
		}
		fmt.Printf("Đã tạo mới account 1: %s\n", acc1.Address)
	}

	// Khởi tạo account thứ hai với số dư ban đầu
	acc2, err := NewAccountFromPublicKey(pubKey2, 1000)
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo account 2: %v", err)
	}

	// Kiểm tra account 2 đã tồn tại chưa
	existingAcc2, err := bc.GetAccountFromTrie(acc2.Address)
	if err == nil && existingAcc2 != nil {
		fmt.Printf("Account 2 đã tồn tại: %s\n", acc2.Address)
	} else {
		// Lưu account 2 vào trie nếu chưa tồn tại
		if err := bc.SaveAccountToTrie(acc2); err != nil {
			return fmt.Errorf("lỗi lưu account 2: %v", err)
		}
		fmt.Printf("Đã tạo mới account 2: %s\n", acc2.Address)
	}

	// Khởi tạo account thứ ba với số dư ban đầu
	acc3, err := NewAccountFromPublicKey(pubKey3, 1000)
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo account 3: %v", err)
	}

	// Kiểm tra account 3 đã tồn tại chưa
	existingAcc3, err := bc.GetAccountFromTrie(acc3.Address)
	if err == nil && existingAcc3 != nil {
		fmt.Printf("Account 3 đã tồn tại: %s\n", acc3.Address)
	} else {
		// Lưu account 3 vào trie nếu chưa tồn tại
		if err := bc.SaveAccountToTrie(acc3); err != nil {
			return fmt.Errorf("lỗi lưu account 3: %v", err)
		}
		fmt.Printf("Đã tạo mới account 3: %s\n", acc3.Address)
	}

	return nil
}

// TransferBalance chuyển tiền giữa các tài khoản
func (bc *Blockchain) TransferBalance(from *Account, to *Account, amount uint64) error {
	if from.Balance < amount {
		return fmt.Errorf("số dư không đủ")
	}

	from.Balance -= amount
	to.Balance += amount

	// Lưu các thay đổi
	if err := bc.SaveAccountToTrie(from); err != nil {
		return fmt.Errorf("lỗi khi cập nhật tài khoản người gửi: %v", err)
	}

	if err := bc.SaveAccountToTrie(to); err != nil {
		return fmt.Errorf("lỗi khi cập nhật tài khoản người nhận: %v", err)
	}

	return nil
}
