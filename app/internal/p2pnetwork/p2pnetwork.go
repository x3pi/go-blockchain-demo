package p2pnetwork

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"time"

	"main/internal/blockchain"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type P2PNetwork struct {
	Config     *Config
	Server     *p2p.Server
	PrivateKey *ecdsa.PrivateKey
	blockchain *blockchain.Blockchain // Thêm trường này
}

type Config struct {
	PrivateKeyHex string
	Nodes         []string
	MaxPeers      int
	Name          string
	ListenAddr    string
}

func NewP2PNetwork(config *Config) (*P2PNetwork, error) {
	nodekey, err := crypto.HexToECDSA(config.PrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("không thể tạo khóa node từ hex: %v", err)
	}

	return &P2PNetwork{
		Config:     config,
		PrivateKey: nodekey,
		blockchain: blockchain.GetInstance(),
	}, nil
}

func (p *P2PNetwork) Start() error {
	var nodes []*enode.Node
	for _, nodeAddr := range p.Config.Nodes {
		nodes = append(nodes, enode.MustParse(nodeAddr))
	}

	cfg := p2p.Config{
		PrivateKey: p.PrivateKey,
		MaxPeers:   p.Config.MaxPeers,
		Name:       p.Config.Name,
		ListenAddr: p.Config.ListenAddr,
	}

	proto := p2p.Protocol{
		Name:    "ping",
		Version: 1,
		Length:  1,
		Run:     p.runPing,
	}

	blockProto := p2p.Protocol{
		Name:    "block",
		Version: 1,
		Length:  4, // Tăng length lên 2 để hỗ trợ 2 loại tin nhắn
		Run:     p.runBlockProtocol,
	}

	cfg.Protocols = []p2p.Protocol{proto, blockProto}

	p.Server = &p2p.Server{Config: cfg}
	if err := p.Server.Start(); err != nil {
		return fmt.Errorf("không thể khởi động server: %v", err)
	}

	for _, node := range nodes {
		p.Server.AddPeer(node)
	}

	return nil
}

func (p *P2PNetwork) Stop() {
	if p.Server != nil {
		p.Server.Stop()
	}
}

func (p *P2PNetwork) runPing(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	for {
		err := p2p.Send(rw, 0, "PING")
		if err != nil {
			return fmt.Errorf("lỗi khi gửi PING đến %v: %v", peer.ID(), err)
		}

		msg, err := rw.ReadMsg()
		if err != nil {
			return fmt.Errorf("lỗi khi đọc tin nhắn từ %v: %v", peer.ID(), err)
		}

		var ping string
		err = msg.Decode(&ping)
		if err != nil {
			return fmt.Errorf("lỗi khi giải mã tin nhắn từ %v: %v", peer.ID(), err)
		}

		log.Printf("Nhận %s từ %v\n", ping, peer.ID())
		time.Sleep(20 * time.Second)
	}
}

func (p *P2PNetwork) runBlockProtocol(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	go p.handleIncomingBlockMessages(peer, rw)

	for {
		now := time.Now()
		nextRun := now.Truncate(time.Minute).Add(30 * time.Second)
		if nextRun.Before(now) {
			nextRun = nextRun.Add(time.Minute)
		}
		time.Sleep(nextRun.Sub(now))

		// Gửi yêu cầu lấy block với xử lý lỗi tốt hơn
		blockID := fmt.Sprintf("block_%d", time.Now().Unix())
		request := BlockRequest{
			Type:    "request",
			BlockID: blockID,
		}
		if err := p2p.Send(rw, BlockRequestMsg, request); err != nil {
			log.Printf("Lỗi khi gửi yêu cầu block đến %v: %v\n", peer.ID(), err)
			continue
		}
		log.Printf("Đã gửi yêu cầu block %s đến %v thành công\n", blockID, peer.ID())

		// Gửi yêu cầu last block với xử lý lỗi tốt hơn
		lastBlockRequest := BlockRequest{
			Type: "last_block",
		}
		if err := p2p.Send(rw, LastBlockRequestMsg, lastBlockRequest); err != nil {
			log.Printf("Lỗi khi gửi yêu cầu last block đến %v: %v\n", peer.ID(), err)
			return err // Return error to potentially restart the protocol
		}
		log.Printf("Đã gửi yêu cầu last block đến %v thành công\n", peer.ID())
	}
}

func (p *P2PNetwork) handleIncomingBlockMessages(peer *p2p.Peer, rw p2p.MsgReadWriter) {
	for {
		msg, err := rw.ReadMsg()
		if err != nil {
			log.Printf("Lỗi khi đọc tin nhắn từ %v: %v\n", peer.ID(), err)
			return
		}

		switch msg.Code {
		case BlockRequestMsg:
			var request BlockRequest
			if err := msg.Decode(&request); err != nil {
				log.Printf("Lỗi khi giải mã yêu cầu block: %v\n", err)
				continue
			}
			// Add logging for block request
			log.Printf("Nhận được yêu cầu block từ %v:\n", peer.ID())
			log.Printf("  Type: %s\n", request.Type)
			log.Printf("  BlockID: %s\n", request.BlockID)
			// Xử lý yêu cầu và gửi block
			currentTime := time.Now().Unix()
			block := blockchain.Block{
				BlockHeader: blockchain.BlockHeader{
					Version:             uint64(1),
					PreviousBlockHeader: "previous_hash",
					MerkleRoot:          "merkle_root",
					Time:                uint64(currentTime), // Chuyển đổi int64 sang uint64
					Signature:           "signature",
				},
				Index: uint64(0),
				Txns:  []string{"transaction1", "transaction2"},
			}
			err = p2p.Send(rw, BlockResponseMsg, block)
			if err != nil {
				log.Printf("Lỗi khi gửi block đến %v: %v\n", peer.ID(), err)
			}

		case LastBlockRequestMsg:
			lastBlock, err := p.blockchain.GetLastBlock()
			if err != nil {
				log.Printf("Lỗi khi lấy last block: %v\n", err)
				continue
			}
			err = p2p.Send(rw, LastBlockResponseMsg, lastBlock)
			if err != nil {
				log.Printf("Lỗi khi gửi last block đến %v: %v\n", peer.ID(), err)
			}

		case BlockResponseMsg:
			var block blockchain.Block
			if err := msg.Decode(&block); err != nil {
				log.Printf("Lỗi khi giải mã block: %v\n", err)
				continue
			}
			// Xử lý block thông thường
			p.handleBlockResponse(peer, block)

		case LastBlockResponseMsg:
			var block blockchain.Block
			if err := msg.Decode(&block); err != nil {
				log.Printf("Lỗi khi giải mã last block: %v\n", err)
				continue
			}
			// Xử lý last block
			p.handleLastBlockResponse(peer, block)
		}
	}
}

// Định nghĩa cấu trúc cho tin nhắn yêu cầu
type BlockRequest struct {
	Type    string `json:"type"`
	BlockID string `json:"block_id"`
}

// Định nghĩa các hằng số cho các loại tin nhắn
const (
	BlockRequestMsg uint64 = iota
	BlockResponseMsg
	LastBlockRequestMsg
	LastBlockResponseMsg
)

// Thêm các hàm xử lý mới
func (p *P2PNetwork) handleBlockResponse(peer *p2p.Peer, block blockchain.Block) {
	log.Printf("Nhận được block từ %v:\n", peer.ID())
	p.logBlockInfo(block)
}

func (p *P2PNetwork) handleLastBlockResponse(peer *p2p.Peer, block blockchain.Block) {
	log.Printf("Nhận được last block từ %v:\n", peer.ID())
	p.logBlockInfo(block)
	// TODO: Thêm xử lý đặc biệt cho last block ở đây
}

func (p *P2PNetwork) logBlockInfo(block blockchain.Block) {
	log.Printf("  Index: %d\n", block.Index)
	log.Printf("  Version: %d\n", block.BlockHeader.Version)
	log.Printf("  PreviousBlockHeader: %s\n", block.BlockHeader.PreviousBlockHeader)
	log.Printf("  MerkleRoot: %s\n", block.BlockHeader.MerkleRoot)
	log.Printf("  Time: %d\n", block.BlockHeader.Time)
	log.Printf("  Signature: %s\n", block.BlockHeader.Signature)
	log.Printf("  Transactions: %v\n", block.Txns)
}
