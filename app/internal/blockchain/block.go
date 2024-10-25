package blockchain

// BlockHeader đại diện cho tiêu đề khối
type BlockHeader struct {
	Version             int    // Phiên bản
	PreviousBlockHeader string // băm tiêu đề khối trước
	MerkleRoot          string // băm gốc merkle
	Time                int64  // thời gian epoch Unix
}

// Block đại diện cho một khối
type Block struct {
	BlockHeader BlockHeader // Tiêu đề khối
	Index       int         // Chỉ số khối
	Txns        []string    // Giao dịch thô trong khối
}
