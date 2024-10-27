package blockchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

var (
	blockchain []Block
	tr         *trie.Trie // Moved tr to package-level variable
)

func GetGenesisBlock() Block {
	blockHeader := BlockHeader{
		Version:             1,
		PreviousBlockHeader: "",
		MerkleRoot:          "0x1bc3300000000000000000000000000000000000000000",
		Time:                uint64(time.Now().Unix()),
	}
	return Block{BlockHeader: blockHeader, Index: 0, Txns: nil}
}

func GetLatestBlock() Block {
	return blockchain[len(blockchain)-1]
}

func AddBlock(newBlock Block) {
	prevBlock := GetLatestBlock()
	if prevBlock.Index < newBlock.Index && newBlock.BlockHeader.PreviousBlockHeader == prevBlock.BlockHeader.MerkleRoot {
		blockchain = append(blockchain, newBlock)
	}
}

func GetBlock(index int) *Block {
	if len(blockchain)-1 >= index {
		return &blockchain[index]
	}
	return nil
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
func SaveBlockToTrie(tr *trie.Trie, block Block, index int) error {
	blockJSON, err := json.Marshal(block)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("block_%d", index))
	tr.Update(key, blockJSON)
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
		return Block{}, err
	}
	if value == nil {
		return Block{}, fmt.Errorf("không tìm thấy block") // Changed to lowercase
	}
	var block Block
	err = json.Unmarshal(value, &block)
	return block, err
}

func Init() {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Lỗi khi lấy thư mục hiện tại: %v\n", err)
		return
	}
	dbPath := filepath.Join(currentDir, "leveldb_data")

	db, err := InitLevelDB(dbPath)
	if err != nil {
		fmt.Printf("Lỗi khi tạo LevelDB: %v\n", err)
		return
	}
	defer db.Close()

	trieDB := triedb.NewDatabase(rawdb.NewDatabase(db), &triedb.Config{})
	rootKey := []byte("ROOT_HASH")
	rootHash, err := db.Get(rootKey)

	if err == errors.ErrNotFound {
		tr, err = CreateNewTrie(trieDB) // Initialize tr here
		if err != nil {
			fmt.Printf("Lỗi khi tạo trie mới: %v\n", err)
			return
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
		if err := SaveBlockToTrie(tr, block, 0); err != nil {
			fmt.Printf("Lỗi khi lưu block: %v\n", err)
			return
		}

		root, nodes := tr.Commit(false)
		if err := trieDB.Update(root, common.Hash{}, 0, trienode.NewWithNodeSet(nodes), nil); err != nil {
			fmt.Printf("Lỗi khi cập nhật trie database: %v\n", err)
			return
		}

		if err := trieDB.Commit(root, false); err != nil {
			fmt.Printf("Lỗi khi commit trie database: %v\n", err)
			return
		}

		if err := db.Put(rootKey, root[:]); err != nil {
			fmt.Printf("Lỗi khi lưu root hash: %v\n", err)
			return
		}
		fmt.Printf("Đã lưu trie mới vào cơ sở dữ liệu, root hash: %x\n", root)

		tr, err = CreateNewTrie(trieDB)
		if err != nil {
			fmt.Printf("Lỗi khi tạo trie mới sau khi commit: %v\n", err)
			return
		}
	} else if err != nil {
		fmt.Printf("Lỗi khi lấy root hash từ database: %v\n", err)
		return
	} else {
		tr, err = RestoreTrieFromRootHash(rootHash, trieDB) // Restore tr here
		if err != nil {
			fmt.Printf("Lỗi khi khôi phục trie: %v\n", err)
			return
		}
		fmt.Println("Đã khôi phục trie từ cơ sở dữ liệu")
	}

	key := []byte("block_0")
	block, err := GetBlockFromTrie(tr, key) // Use tr here
	if err != nil {
		fmt.Printf("Lỗi khi truy xuất block_0: %v\n", err)
	} else {
		fmt.Printf("Đã truy xuất block_0: %+v\n", block)
	}

	fmt.Printf("Root hash của trie: %x\n", tr.Hash())
}
