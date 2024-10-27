package blockchain

// BlockHeader đại diện cho tiêu đề khối
type BlockHeader struct {
	Version             uint64 // Phiên bản
	PreviousBlockHeader string // băm tiêu đề khối trước
	MerkleRoot          string // băm gốc merkle
	Time                uint64 // thời gian epoch Unix
	Signature           string // chữ ký của người đề xuất khối
}

// Block đại diện cho một khối
type Block struct {
	BlockHeader BlockHeader // Tiêu đề khối
	Index       uint64      // Chỉ số khối
	Txns        []string    // Giao dịch thô trong khối
}
