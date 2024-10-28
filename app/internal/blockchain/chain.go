package blockchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

// Define constants for database keys
const (
	ROOT_HASH     = "ROOT_HASH"
	CURRENT_BLOCK = "CURRENT_BLOCK"
	LAST_BLOCK    = "LAST_BLOCK"
)

type Blockchain struct {
	db     *leveldb.Database
	tr     *trie.Trie
	mut    sync.RWMutex // để thread-safe
	Config Config       // Thay đổi từ config thành Config để có thể truy cập từ bên ngoài
}

type Node struct {
	PublicKey string `json:"publicKey"`
	URL       string `json:"url"`
	Index     int    `json:"index"` // Add this line
}

// Định nghĩa cấu trúc cho file cấu hình
type Config struct {
	Nodes         []Node `json:"nodes"` // Thay đổi từ []string thành []Node
	PrivateKeyHex string `json:"privateKeyHex"`
	Index         int    `json:"index"` // Thêm trường index
}

var (
	instance *Blockchain
	once     sync.Once
)

// GetInstance trả về instance singleton của Blockchain
func GetInstance() *Blockchain {
	once.Do(func() {
		instance = &Blockchain{}
	})
	return instance
}

// Init khởi tạo blockchain
func (bc *Blockchain) Init() error {
	bc.mut.Lock()
	defer bc.mut.Unlock()

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("lỗi khi lấy thư mục hiện tại: %v", err)
	}
	dbPath := filepath.Join(currentDir, "leveldb_data")

	bc.db, err = InitLevelDB(dbPath)
	if err != nil {
		return fmt.Errorf("lỗi khi tạo LevelDB: %v", err)
	}

	trieDB := triedb.NewDatabase(rawdb.NewDatabase(bc.db), &triedb.Config{})
	rootKey := []byte(ROOT_HASH) // Use the constant for ROOT_HASH
	rootHash, err := bc.db.Get(rootKey)
	fmt.Printf("Root hash: %x\n", rootHash) // Print the root hash
	if err == errors.ErrNotFound {
		bc.tr, err = CreateNewTrie(trieDB) // Initialize tr here
		if err != nil {
			fmt.Printf("Lỗi khi tạo trie mới: %v\n", err)
			return nil
		}
		fmt.Println("Đã tạo trie mới")

		// Tạo genesis block
		block := Block{
			BlockHeader: BlockHeader{
				Version:    uint64(1),
				MerkleRoot: fmt.Sprintf("block_%d", 0),
				Time:       uint64(time.Now().Unix()),
			},
			Index: uint64(0),
			Txns:  []string{fmt.Sprintf("tx_%d_1", 0), fmt.Sprintf("tx_%d_2", 0)},
		}
		if err := SaveBlockToTrie(block, 0, bc); err != nil {
			fmt.Printf("Lỗi khi lưu block: %v\n", err)
			return nil
		}

		root, nodes := bc.tr.Commit(false)
		if err := trieDB.Update(root, common.Hash{}, 0, trienode.NewWithNodeSet(nodes), nil); err != nil {
			fmt.Printf("Lỗi khi cập nhật trie database: %v\n", err)
			return nil
		}

		if err := trieDB.Commit(root, false); err != nil {
			fmt.Printf("Lỗi khi commit trie database: %v\n", err)
			return nil
		}

		if err := bc.db.Put(rootKey, root[:]); err != nil {
			fmt.Printf("Lỗi khi lưu root hash: %v\n", err)
			return nil
		}
		fmt.Printf("Đã lưu trie mới vào cơ sở dữ liệu, root hash: %x\n", root)

		bc.tr, err = CreateNewTrie(trieDB)
		if err != nil {
			fmt.Printf("Lỗi khi tạo trie mới sau khi commit: %v\n", err)
			return nil
		}
	} else if err != nil {
		fmt.Printf("Lỗi khi lấy root hash từ database: %v\n", err)
		return nil
	} else {
		bc.tr, err = RestoreTrieFromRootHash(rootHash, trieDB) // Restore tr here
		if err != nil {
			fmt.Printf("Lỗi khi khôi phục trie: %v\n", err)
			return nil
		}
		fmt.Println("Đã khôi phục trie từ cơ sở dữ liu")
	}

	key := []byte("block_0")
	block, err := bc.GetBlockFromTrie(key) // Use tr here
	if err != nil {
		fmt.Printf("Lỗi khi truy xuất block_0: %v\n", err)
	} else {
		fmt.Printf("Đã truy xuất block_0: %+v\n", block)
	}

	fmt.Printf("Root hash của trie: %x\n", bc.tr.Hash())

	lastBlock, err := bc.GetLastBlock()
	if err != nil {
		fmt.Printf("Lỗi khi lấy LAST_BLOCK: %v\n", err)
	} else {
		fmt.Printf("LAST_BLOCK: %+v\n", lastBlock)
	}

	currentBlock, err := bc.GetCurrentBlock()
	if err != nil {
		fmt.Printf("Lỗi khi lấy CURRENT_BLOCK: %v\n", err)
	} else {
		fmt.Printf("CURRENT_BLOCK: %+v\n", currentBlock)
	}

	// Thêm bước khởi tạo config
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("không thể đọc file cấu hình: %v", err)
	}

	var config Config // Khai báo biến config
	if err := json.Unmarshal(configFile, &config); err != nil {
		return fmt.Errorf("không thể phân tích file cấu hình: %v", err)
	}
	bc.Config = config // Gán giá trị cho bc.config
	return nil
}

// GetLastBlock thread-safe
func (bc *Blockchain) GetLastBlock() (Block, error) {
	// Bỏ debug log để tránh overhead không cần thiết
	blockJSON, err := bc.db.Get([]byte(LAST_BLOCK))
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi lấy LAST_BLOCK: %v", err)
	}

	var block Block
	err = json.Unmarshal(blockJSON, &block)
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi giải mã LAST_BLOCK: %v", err)
	}

	return block, nil
}

// Save the current block to CURRENT_BLOCK in the database
func (bc *Blockchain) SaveCurrentBlock(block Block) error {
	blockJSON, err := json.Marshal(block)
	if err != nil {
		return err
	}
	if err := bc.db.Put([]byte(CURRENT_BLOCK), blockJSON); err != nil {
		return fmt.Errorf("lỗi khi lưu CURRENT_BLOCK: %v", err)
	}
	return nil
}

// Save the current block to LAST_BLOCK in the database
func (bc *Blockchain) SaveLastBlock(block Block) error {
	blockJSON, err := json.Marshal(block)
	if err != nil {
		return err
	}
	if err := bc.db.Put([]byte(LAST_BLOCK), blockJSON); err != nil {
		return fmt.Errorf("lỗi khi lưu LAST_BLOCK: %v", err)
	}
	return nil
}

// Get the current block from CURRENT_BLOCK in the database
func (bc *Blockchain) GetCurrentBlock() (Block, error) {
	blockJSON, err := bc.db.Get([]byte(CURRENT_BLOCK))
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi lấy CURRENT_BLOCK: %v", err)
	}
	var block Block
	err = json.Unmarshal(blockJSON, &block)
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi giải mã CURRENT_BLOCK: %v", err)
	}
	return block, nil
}

// Retrieve the last block from LAST_BLOCK in the database
func (bc *Blockchain) RetrieveLastBlock() (Block, error) {
	blockJSON, err := bc.db.Get([]byte(LAST_BLOCK))
	if err != nil {
		return Block{}, fmt.Errorf("error retrieving LAST_BLOCK: %v", err)
	}
	var block Block
	err = json.Unmarshal(blockJSON, &block)
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi giải mã LAST_BLOCK: %v", err)
	}
	return block, nil
}

func GetGenesisBlock() Block {
	blockHeader := BlockHeader{
		Version:             1,
		PreviousBlockHeader: "",
		MerkleRoot:          "0x1bc3300000000000000000000000000000000000000",
		Time:                uint64(time.Now().Unix()),
	}
	return Block{BlockHeader: blockHeader, Index: 0, Txns: nil}
}

// Khởi tạo LevelDB
func InitLevelDB(dbPath string) (*leveldb.Database, error) {
	return leveldb.New(dbPath, 0, 0, "", false)
}

// Tạo một trie mới
func CreateNewTrie(db *triedb.Database) (*trie.Trie, error) {
	return trie.New(trie.TrieID(common.Hash{}), db)
}

// Lưu block vào trie
func SaveBlockToTrie(block Block, index int, bc *Blockchain) error {
	blockJSON, err := json.Marshal(block)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("block_%d", index))
	bc.tr.Update(key, blockJSON)
	fmt.Printf("Đã lưu %s\n", fmt.Sprintf("block_%d", index))

	root, nodes := bc.tr.Commit(false)
	trieDB := triedb.NewDatabase(rawdb.NewDatabase(bc.db), &triedb.Config{})
	if err := trieDB.Update(root, common.Hash{}, 0, trienode.NewWithNodeSet(nodes), nil); err != nil {
		return fmt.Errorf("lỗi khi cập nhật trie database: %v", err)
	}

	if err := trieDB.Commit(root, false); err != nil {
		return fmt.Errorf("lỗi khi commit trie database: %v", err)
	}

	if err := bc.db.Put([]byte(ROOT_HASH), root[:]); err != nil {
		return fmt.Errorf("lỗi khi lưu root hash: %v", err)
	}
	if index > 1 {
		curentBlock, err := bc.GetCurrentBlock()
		if err != nil {
			return fmt.Errorf("lỗi khi lấy LAST_BLOCK: %v", err)
		}

		lastBlock, err := bc.GetLastBlock()
		if err != nil {
			return fmt.Errorf("lỗi khi lấy LAST_BLOCK: %v", err)
		}

		if block.Index == curentBlock.Index+1 {
			// Cập nhật CURRENT_BLOCK
			if err := bc.SaveCurrentBlock(block); err != nil {
				return fmt.Errorf("lỗi khi lưu CURRENT_BLOCK: %v", err)
			}
			fmt.Printf("Cập nhật CURRENT_BLOCK %s\n", fmt.Sprintf("block_%d", index))

		}

		if block.Index > lastBlock.Index {
			// Cập nhật LAST_BLOCK
			if err := bc.SaveLastBlock(block); err != nil {
				return fmt.Errorf("lỗi khi lưu LAST_BLOCK: %v", err)
			}
			fmt.Printf("Cập nhật LAST_BLOCK %s\n", fmt.Sprintf("block_%d", index))
		}

	} else {
		if err := bc.SaveCurrentBlock(block); err != nil {
			return fmt.Errorf("lỗi khi lưu CURRENT_BLOCK: %v", err)
		}

		if err := bc.SaveLastBlock(block); err != nil {
			return fmt.Errorf("lỗi khi lưu LAST_BLOCK: %v", err)
		}
		fmt.Printf("Cập nhật LAST_BLOCK %s\n", fmt.Sprintf("block_%d", index))
		fmt.Printf("Cập nhật CURRENT_BLOCK %s\n", fmt.Sprintf("block_%d", index))

	}

	return nil
}

// Khôi phục trie từ root hash
func RestoreTrieFromRootHash(rootHash []byte, trieDB *triedb.Database) (*trie.Trie, error) {
	return trie.New(trie.TrieID(common.BytesToHash(rootHash)), trieDB)
}

// Truy xuất block từ trie
func (bc *Blockchain) GetBlockFromTrie(key []byte) (Block, error) { // Updated method to be a receiver method
	value, err := bc.tr.Get(key) // Use bc.tr instead of instance.tr
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi truy xuất dữ liệu: %v", err)
	}
	if value == nil {
		return Block{}, fmt.Errorf("không tìm thấy block với key: %s", string(key))
	}
	var block Block
	if err = json.Unmarshal(value, &block); err != nil {
		return Block{}, fmt.Errorf("lỗi khi giải mã block: %v", err)
	}
	return block, nil
}

// CheckProposalCondition kiểm tra điều kiện để đề xuất khối mới
func (bc *Blockchain) CheckProposalCondition() bool {
	// Kiểm tra điều kiện, ví dụ: số lượng giao dch, thời gian giữa các khối, v.v.
	// Đây chỉ là một ví dụ đơn giản, bạn có thể thay đổi theo yêu cầu của mình.
	lastBlock, err := bc.GetLastBlock()
	if err != nil {
		fmt.Printf("Lỗi khi lấy LAST_BLOCK: %v\n", err)
		return false
	}

	currentBlock, err := bc.GetCurrentBlock()
	if err != nil {
		fmt.Printf("Lỗi khi lấy CURRENT_BLOCK: %v\n", err)
		return false
	}

	if currentBlock.Index < lastBlock.Index {
		fmt.Println("Cần cập nhật đầy đủ trước khi đề xuất")
		return false
	}

	if currentBlock.Index < lastBlock.Index {
		fmt.Println("Cần cập nhật đầy đủ trước khi đề xuất")
		return false
	}

	if (currentBlock.Index+1-uint64(bc.Config.Index))%3 != 0 {
		fmt.Println("Không phải block có quyền được đề xuất")
		return false
	}

	// Lấy block liền kề bằng GetBlockFromTrie
	key := []byte(fmt.Sprintf("block_%d", currentBlock.Index-1)) // Tạo key cho block liền kề
	preBlock, err := bc.GetBlockFromTrie(key)                    // Sử dụng GetBlockFromTrie để lấy block liền kề
	if err != nil {
		fmt.Printf("Lỗi khi lấy block liền kề: %v\n", err)
		return false
	}

	if preBlock.BlockHeader.Signature == "" && currentBlock.BlockHeader.Signature == "" {
		fmt.Printf("2 block gần nhất không được ký: %v\n", err)
		return false
	}
	// Thêm các điều kiện khác nếu cần
	return true
}

// ProposeNewBlock đề xuất một khối mới và trả về khối được đề xuất nếu có
func (bc *Blockchain) ProposeNewBlock() (Block, error) { // Thay đổi kiểu trả về
	bc.mut.Lock()
	defer bc.mut.Unlock()

	// Kiểm tra điều kiện để đề xuất khối mới

	// Tạo khối mới
	currentBlock, err := bc.GetCurrentBlock()
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi lấy CURRENT_BLOCK: %v", err)
	}

	if !bc.CheckProposalCondition() {
		// Nếu là block chưa được ký
		if currentBlock.Index > 2 && (currentBlock.Index-uint64(bc.Config.Index))%3 == 0 && currentBlock.BlockHeader.Signature == "" {
			sg, err := SignMerkleRoot(bc.Config.PrivateKeyHex, fmt.Sprintf("block_%d", currentBlock.Index))
			if err != nil {
				return Block{}, fmt.Errorf("lỗi khi tạo chữ ký: %v", err)
			}
			block := Block{
				BlockHeader: BlockHeader{
					Version:             currentBlock.BlockHeader.Version,
					MerkleRoot:          fmt.Sprintf("block_%d", currentBlock.Index),
					PreviousBlockHeader: currentBlock.BlockHeader.PreviousBlockHeader,
					Time:                uint64(time.Now().Unix()),
					Signature:           sg, // Gán chữ ký vào đây
				},
				Index: currentBlock.Index + 1,
				Txns:  []string{}, // Thêm giao dịch nếu cần

			}
			return block, nil

		}
	}

	// Tạo chữ ký cho MerkleRoot
	signature, err := SignMerkleRoot(bc.Config.PrivateKeyHex, fmt.Sprintf("block_%d", currentBlock.Index+1))
	if err != nil {
		return Block{}, fmt.Errorf("lỗi khi tạo chữ ký: %v", err)
	}

	newBlock := Block{
		BlockHeader: BlockHeader{
			Version:             currentBlock.BlockHeader.Version,
			PreviousBlockHeader: currentBlock.BlockHeader.MerkleRoot,
			MerkleRoot:          fmt.Sprintf("block_%d", currentBlock.Index+1),
			Time:                uint64(time.Now().Unix()),
			Signature:           signature, // Gán chữ ký vào đây
		},
		Index: currentBlock.Index + 1,
		Txns:  []string{}, // Thêm giao dịch nếu cần
	}

	// Lưu khối mới vào trie
	if err := SaveBlockToTrie(newBlock, int(newBlock.Index), bc); err != nil {
		return Block{}, fmt.Errorf("lỗi khi lưu khối mới: %v", err)
	}

	fmt.Printf("Đã đề xuất khối mới: %+v\n", newBlock)
	return newBlock, nil // Trả khối được đề xuất
}

// SignMerkleRoot signs a message using the private key
func SignMerkleRoot(privateKeyHex string, message string) (string, error) {
	// Convert hex private key to ECDSA private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %v", err)
	}

	// Hash the message
	messageHash := crypto.Keccak256Hash([]byte(message))

	// Sign the hash
	signature, err := crypto.Sign(messageHash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("error signing message: %v", err)
	}

	// Convert signature to hex string
	return hexutil.Encode(signature), nil
}

// VerifySignature verifies if the signature is valid
func VerifySignature(publicKeyHex string, message string, signatureHex string) (bool, error) {
	// Decode the signature from hex
	signature, err := hexutil.Decode(signatureHex)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %v", err)
	}

	// Hash the message
	messageHash := crypto.Keccak256Hash([]byte(message))

	// Decode public key from hex
	publicKeyBytes, err := hexutil.Decode(publicKeyHex)
	if err != nil {
		return false, fmt.Errorf("invalid public key hex: %v", err)
	}

	// Convert bytes to public key
	publicKey, err := crypto.UnmarshalPubkey(publicKeyBytes)
	if err != nil {
		return false, fmt.Errorf("invalid public key: %v", err)
	}

	// Verify the signature
	sigPublicKey, err := crypto.Ecrecover(messageHash.Bytes(), signature)
	if err != nil {
		return false, fmt.Errorf("error recovering public key: %v", err)
	}

	matches := bytes.Equal(crypto.FromECDSAPub(publicKey), sigPublicKey)
	return matches, nil
}

// HandleNewBlock xử lý khi nhận được một block mới
func (bc *Blockchain) HandleNewBlock(newBlock Block) error {
	bc.mut.Lock()
	defer bc.mut.Unlock()

	// Thêm kiểm tra xem newBlock.Index đã tồn tại chưa
	existingBlock, _ := bc.GetBlockFromTrie([]byte(fmt.Sprintf("block_%d", newBlock.Index)))
	if existingBlock.BlockHeader.Signature != "" {
		return fmt.Errorf("Block đã tồn tại: chỉ số = %d", newBlock.Index) // Thêm thông báo lỗi nếu block đã tồn tại
	}

	node, err := bc.GetNodeConfigByIndex(int(newBlock.Index % 3))
	if err != nil {
		return fmt.Errorf("lỗi khi lấy node: %v", err)
	}

	if _, err := VerifySignature(node.PublicKey, fmt.Sprintf("block_%d", newBlock.Index), newBlock.BlockHeader.Signature); err != nil {
		return fmt.Errorf("lỗi khi kiểm tra chữ ký: %v", err)
	}

	// Lưu block mới vào trie
	if err := SaveBlockToTrie(newBlock, int(newBlock.Index), bc); err != nil {
		return fmt.Errorf("lỗi khi lưu block mới vào trie: %v", err)
	}

	fmt.Printf("Đã xử lý block mới: %+v\n", newBlock)
	return nil
}

// GetNodeConfigByIndex retrieves the node configuration by the Index field
func (bc *Blockchain) GetNodeConfigByIndex(index int) (Node, error) {
	for _, node := range bc.Config.Nodes {
		if node.Index == index {
			return node, nil
		}
	}
	return Node{}, fmt.Errorf("no node found with index: %d", index)
}
