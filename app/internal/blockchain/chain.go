package blockchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
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
	db  *leveldb.Database
	tr  *trie.Trie
	mut sync.RWMutex // để thread-safe
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
				Version:             uint64(1),
				PreviousBlockHeader: fmt.Sprintf("prev_hash_%d", 0),
				MerkleRoot:          fmt.Sprintf("merkle_root_%d", 0),
				Time:                uint64(1630000000 + uint64(0)*600),
			},
			Index: uint64(0),
			Txns:  []string{fmt.Sprintf("tx_%d_1", 0), fmt.Sprintf("tx_%d_2", 0)},
		}
		if err := SaveBlockToTrie(block, 0, bc); err != nil {
			fmt.Printf("Lỗi khi lưu block: %v\n", err)
			return nil
		}

		fmt.Println("Bắt đầu lưu genesis block...")
		if err := bc.SaveCurrentBlock(block); err != nil {
			fmt.Printf("Lỗi khi lưu CURRENT_BLOCK: %v\n", err)
			return err // Thêm return err để dừng nếu có lỗi
		}
		fmt.Println("Đã lưu CURRENT_BLOCK thành công")

		if err := bc.SaveLastBlock(block); err != nil {
			fmt.Printf("Lỗi khi lưu LAST_BLOCK: %v\n", err)
			return err // Thêm return err để dừng nếu có lỗi
		}
		fmt.Println("Đã lưu LAST_BLOCK thành công")

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
	block, err := GetBlockFromTrie(bc.tr, key) // Use tr here
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
	return nil
}

// Khôi phục trie từ root hash
func RestoreTrieFromRootHash(rootHash []byte, trieDB *triedb.Database) (*trie.Trie, error) {
	return trie.New(trie.TrieID(common.BytesToHash(rootHash)), trieDB)
}

// Truy xuất block từ trie
func GetBlockFromTrie(tr *trie.Trie, key []byte) (Block, error) {
	value, err := tr.Get(key)
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
