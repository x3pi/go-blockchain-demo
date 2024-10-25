package blockchain

import (
	"time"
)

var blockchain []Block

func GetGenesisBlock() Block {
	blockHeader := BlockHeader{
		Version:             1,
		PreviousBlockHeader: "",
		MerkleRoot:          "0x1bc3300000000000000000000000000000000000000000000",
		Time:                time.Now().Unix(),
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

func Init() {
	blockchain = []Block{GetGenesisBlock()}
}
