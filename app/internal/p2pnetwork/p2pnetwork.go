package p2pnetwork

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"sync"
	"time"

	"main/internal/blockchain"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type P2PNetwork struct {
	Config         *Config
	Server         *p2p.Server
	PrivateKey     *ecdsa.PrivateKey
	blockchain     *blockchain.Blockchain
	txChannels     map[string]chan *blockchain.Transaction
	txChannelMutex sync.RWMutex
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
		Config:         config,
		PrivateKey:     nodekey,
		blockchain:     blockchain.GetInstance(),
		txChannels:     make(map[string]chan *blockchain.Transaction),
		txChannelMutex: sync.RWMutex{},
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

	// proto := p2p.Protocol{
	// 	Name:    "ping",
	// 	Version: 1,
	// 	Length:  1,
	// 	Run:     p.runPing,
	// }

	blockProto := p2p.Protocol{
		Name:    "block",
		Version: 1,
		Length:  7,
		Run:     p.runBlockProtocol,
	}

	cfg.Protocols = []p2p.Protocol{blockProto}

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

// func (p *P2PNetwork) runPing(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
// 	for {
// 		err := p2p.Send(rw, 0, "PING")
// 		if err != nil {
// 			return fmt.Errorf("lỗi khi gửi PING đến %v: %v", peer.ID(), err)
// 		}

// 		msg, err := rw.ReadMsg()
// 		if err != nil {
// 			return fmt.Errorf("lỗi khi đọc tin nhắn từ %v: %v", peer.ID(), err)
// 		}

// 		var ping string
// 		err = msg.Decode(&ping)
// 		if err != nil {
// 			return fmt.Errorf("lỗi khi giải mã tin nhắn từ %v: %v", peer.ID(), err)
// 		}

// 		log.Printf("Nhận %s từ %v\n", ping, peer.ID())
// 		time.Sleep(20 * time.Second)
// 	}
// }

func (p *P2PNetwork) runBlockProtocol(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	// Thêm channel để xử lý transaction từ bên ngoài
	txChan := make(chan *blockchain.Transaction, 100)
	// Đăng ký channel với peer
	p.registerTransactionChannel(peer.ID().String(), txChan)
	defer p.unregisterTransactionChannel(peer.ID().String())

	// Xử lý tin nhắn đến trong một goroutine riêng
	go p.handleIncomingBlockMessages(peer, rw)

	// Thêm goroutine mới để xử lý transaction
	go func() {
		for tx := range txChan {
			if err := p2p.Send(rw, TransactionMsg, tx); err != nil {
				log.Printf("Lỗi khi gửi transaction tới peer %v: %v", peer.ID(), err)
			}
		}
	}()

	// Chạy hai goroutine riêng biệt cho hai chức năng định kỳ
	errChan := make(chan error, 2)

	// Goroutine 1: Chạy khi tới giây thứ 30 của phút
	go func() {
		for {
			now := time.Now()
			nextRun := now.Truncate(time.Minute).Add(30 * time.Second)
			if nextRun.Before(now) {
				nextRun = nextRun.Add(time.Minute)
			}
			time.Sleep(nextRun.Sub(now))
			// Gửi yêu cầu last block
			lastBlockRequest := BlockRequest{
				Type: "last_block",
			}
			if err := p2p.Send(rw, LastBlockRequestMsg, lastBlockRequest); err != nil {
				errChan <- fmt.Errorf("lỗi khi gửi yêu cầu last block: %v", err)
				return
			}
			log.Printf("Đã gửi yêu cầu last block đến %v thành công\n", peer.ID())
			// Gửi yêu cầu đề xuất khối
			block, err := p.blockchain.ProposeNewBlock() // Gửi yêu cầu đề xuất khối
			if err != nil {
				log.Printf("Lỗi khi đề xuất block: %v\n", err)
				return
			}
			if err := p2p.Send(rw, BlockProposalRequestMsg, block); err != nil {
				log.Printf("Lỗi khi gửi đề xuất block đến %v: %v\n", peer.ID(), err)
			}
		}
	}()

	// Goroutine 2: Chức năng mới lặp 20 giây
	go func() {
		for {
			time.Sleep(20 * time.Second)
			currentBlock, err := p.blockchain.GetCurrentBlock() // Updated line
			if err != nil {
				errChan <- fmt.Errorf("lỗi khi lấy CURRENT_BLOCK: %v", err) // Updated line
				return
			}

			lastBlock, err := p.blockchain.GetLastBlock() // Updated line
			if err != nil {
				errChan <- fmt.Errorf("lỗi khi lấy LAST_BLOCK: %v", err) // Updated line
				return
			}
			// Gửi yêu cầu lấy block
			if currentBlock.Index < lastBlock.Index {
				blockID := fmt.Sprintf("block_%d", currentBlock.Index+1)
				request := BlockRequest{
					Type:    "request",
					BlockID: blockID,
				}
				if err := p2p.Send(rw, BlockRequestMsg, request); err != nil {
					errChan <- fmt.Errorf("lỗi khi gửi yêu cầu block: %v", err)
					return
				}
				log.Printf("Đã gửi yêu cầu block %s đến %v thành công\n", blockID, peer.ID())
			}
		}
	}()

	// Đợi và xử lý lỗi từ bất kỳ goroutine nào
	if err := <-errChan; err != nil {
		return err
	}

	return nil
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
			blockIDBytes := []byte(request.BlockID)
			block, err := p.blockchain.GetBlockFromTrie(blockIDBytes)
			if err != nil {
				log.Printf("Lỗi khi lấy block từ trie: %v\n", err)
				return
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

		case BlockProposalRequestMsg:
			var block blockchain.Block
			if err := msg.Decode(&block); err != nil {
				log.Printf("Lỗi khi giải mã last block: %v\n", err)
				continue
			}
			// Xử lý last block
			p.handleBlockProposalRequestMsg(peer, block)

		case BlockProposalResponseMsg:
			var block blockchain.Block
			if err := msg.Decode(&block); err != nil {
				log.Printf("Lỗi khi giải mã block proposal response: %v\n", err)
				continue
			}
			// Xử lý last block
			p.handleLastBlockResponse(peer, block)

		case TransactionMsg:
			var tx blockchain.Transaction
			if err := msg.Decode(&tx); err != nil {
				log.Printf("Lỗi khi giải mã giao dịch: %v\n", err)
				continue
			}

			// Get sender's account and verify signature
			fromAccount, err := p.blockchain.GetAccountFromTrie(tx.From)
			if err != nil {
				log.Printf("Lỗi khi lấy thông tin tài khoản người gửi: %v\n", err)
				continue
			}

			isValid, err := blockchain.VerifyTransactionSignature(&tx, fromAccount.PublicKey)
			if err != nil {
				log.Printf("Lỗi khi xác thực chữ ký giao dịch: %v\n", err)
				continue
			}

			if !isValid {
				log.Printf("Chữ ký giao dịch không hợp lệ\n")
				continue
			}

			// Add to mempool
			if err := p.blockchain.Mempool.AddTransaction(&tx); err != nil {
				log.Printf("Lỗi khi thêm giao dịch vào mempool: %v\n", err)
				continue
			}

			p.blockchain.Mempool.PrintMempool()
			log.Printf("Đã thêm giao dịch %x vào mempool\n", tx.ID)
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
	BlockProposalRequestMsg
	BlockProposalResponseMsg
	TransactionMsg
)

// Thêm các hàm xử lý mới
func (p *P2PNetwork) handleBlockResponse(peer *p2p.Peer, block blockchain.Block) {
	log.Printf("Nhận được block từ %v:\n", peer.ID())
	p.logBlockInfo(block)

	err := p.blockchain.HandleNewBlock(block)
	if err != nil {
		log.Printf("Lỗi khi xử lý block mới: %v\n", err)
		return
	}
}

func (p *P2PNetwork) handleLastBlockResponse(peer *p2p.Peer, block blockchain.Block) {
	log.Printf("Nhận được last block từ %v:\n", peer.ID())
	p.logBlockInfo(block)
	lastBlock, err := p.blockchain.GetLastBlock()
	if err != nil {
		log.Printf("Lỗi khi lấy last block: %v\n", err)
		return
	}
	if block.Index > lastBlock.Index {
		p.blockchain.SaveLastBlock(block)
	}

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

func (p *P2PNetwork) handleBlockProposalRequestMsg(peer *p2p.Peer, block blockchain.Block) {
	log.Printf("Nhận được yêu cầu đề xuất block từ %v:\n", peer.ID())
	p.logBlockInfo(block)

	err := p.blockchain.HandleNewBlock(block)
	if err != nil {
		log.Printf("Lỗi khi xử lý block mới: %v\n", err)
		return
	}

}

// BroadcastTransaction broadcasts a transaction to all connected peers
func (p *P2PNetwork) BroadcastTransaction(tx *blockchain.Transaction) error {
	p.txChannelMutex.RLock()
	defer p.txChannelMutex.RUnlock()

	for peerID, txChan := range p.txChannels {
		select {
		case txChan <- tx:
			log.Printf("Đã gửi transaction tới peer %s", peerID)
		default:
			log.Printf("Channel đầy, bỏ qua gửi transaction tới peer %s", peerID)
		}
	}
	return nil
}

// Thêm các hàm helper để quản lý transaction channels
func (p *P2PNetwork) registerTransactionChannel(peerID string, ch chan *blockchain.Transaction) {
	p.txChannelMutex.Lock()
	defer p.txChannelMutex.Unlock()
	p.txChannels[peerID] = ch
}

func (p *P2PNetwork) unregisterTransactionChannel(peerID string) {
	p.txChannelMutex.Lock()
	defer p.txChannelMutex.Unlock()
	if ch, exists := p.txChannels[peerID]; exists {
		close(ch)
		delete(p.txChannels, peerID)
	}
}
