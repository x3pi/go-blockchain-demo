package p2pnetwork

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"sync"
	"time"

	"main/internal/blockchain"

	"github.com/ethereum/go-ethereum/p2p/enode"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
)

// Group related constants together at the top
const (
	BlockRequestMsg uint64 = iota
	BlockResponseMsg
	LastBlockRequestMsg
	LastBlockResponseMsg
	BlockProposalRequestMsg
	BlockProposalResponseMsg
	TransactionMsg
	TransactionRequestMsg
	TransactionResponseMsg
)

type P2PNetwork struct {
	Config         *Config
	Server         *p2p.Server
	PrivateKey     *ecdsa.PrivateKey
	blockchain     *blockchain.Blockchain
	txChannels     map[string]chan *blockchain.Transaction
	txChannelMutex sync.RWMutex
	peerRWs        map[string]p2p.MsgReadWriter
	peerRWMutex    sync.RWMutex
}

type Config struct {
	PrivateKeyHex string
	Nodes         []string
	MaxPeers      int
	Name          string
	ListenAddr    string
}

type BlockRequest struct {
	Type    string `json:"type"`
	BlockID string `json:"block_id"`
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
		peerRWs:        make(map[string]p2p.MsgReadWriter),
		peerRWMutex:    sync.RWMutex{},
	}, nil
}

func (p *P2PNetwork) Start() error {
	var nodes []*enode.Node
	for _, nodeAddr := range p.Config.Nodes {
		node, err := enode.ParseV4(nodeAddr)
		if err != nil {
			log.Printf("Warning: Failed to parse enode URL %s: %v", nodeAddr, err)
			continue
		}
		nodes = append(nodes, node)
	}

	cfg := p2p.Config{
		PrivateKey:  p.PrivateKey,
		MaxPeers:    p.Config.MaxPeers,
		Name:        p.Config.Name,
		ListenAddr:  p.Config.ListenAddr,
		StaticNodes: nodes,
		NoDiscovery: true,
	}

	pingProto := p2p.Protocol{
		Name:    "ping",
		Version: 1,
		Length:  9,
		Run:     p.runProtocol,
	}

	cfg.Protocols = []p2p.Protocol{pingProto}

	p.Server = &p2p.Server{Config: cfg}
	if err := p.Server.Start(); err != nil {
		return fmt.Errorf("couldn't start server: %v", err)
	}

	// Start connection maintenance
	go p.maintainConnections(nodes)

	return nil
}

func (p *P2PNetwork) Stop() {
	if p.Server != nil {
		p.Server.Stop()
	}
}

func (p *P2PNetwork) runProtocol(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	// Tạo channel để thông báo lỗi từ goroutines
	// Cập nhật peerRWs với kết nối mới
	p.setPeerRW(peer.ID().String(), rw)
	defer p.removePeerRW(peer.ID().String())
	errChan := make(chan error, 4)

	// Goroutine gửi PING
	go func() {
		for {
			log.Printf("==--------------------Gửi yêu cầu last block--------------------==")
			lastBlockRequest := BlockRequest{
				Type: "last_block",
			}

			if err := p2p.Send(rw, LastBlockRequestMsg, lastBlockRequest); err != nil {
				errChan <- fmt.Errorf("lỗi khi gửi yêu cầu last block %v: %v", peer.ID(), err)
				continue
			}
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		for {
			time.Sleep(5 * time.Second)

			log.Printf("==--------------------Đề xuất block--------------------==")

			// Gửi yêu cầu đề xuất khối
			p.blockchain.Mempool.PrintMempool()
			block, err := p.blockchain.ProposeNewBlock()
			if err != nil {
				log.Printf("Lỗi khi đề xuất block: %v\n", err)
				continue
			}

			if block.Index > 0 {
				fmt.Printf("Đề xuất khối index: %d\n", block.Index)
				fmt.Printf("Đề xuất khối: %+v\n", block)
				if err := p2p.Send(rw, BlockProposalRequestMsg, block); err != nil {
					log.Printf("Lỗi khi gửi đề xuất block đến %v: %v\n", peer.ID(), err)
					continue
				}
			}
		}
	}()

	go func() {
		for {
			time.Sleep(1 * time.Second)
			log.Printf("==--------------------Yêu cầu cập nhật block--------------------==")

			currentBlock, err := p.blockchain.GetCurrentBlock()
			if err != nil {
				log.Printf("Lỗi khi lấy CURRENT_BLOCK: %v", err)
				continue
			}

			lastBlock, err := p.blockchain.GetLastBlock()
			if err != nil {
				log.Printf("Lỗi khi lấy LAST_BLOCK: %v", err)
				continue
			}

			log.Printf("Current block và last block %v <> %v:\n", currentBlock.Index, lastBlock.Index)

			if currentBlock.Index < lastBlock.Index {
				blockID := fmt.Sprintf("block_%d", currentBlock.Index+1)
				request := BlockRequest{
					Type:    "request",
					BlockID: blockID,
				}
				log.Printf("Yêu cầu block từ %v:\n", blockID)

				if err := p2p.Send(rw, BlockRequestMsg, request); err != nil {
					log.Printf("Lỗi khi gửi yêu cầu block: %v", err)
					continue
				}
				log.Printf("Đã gửi yêu cầu block %s đến %v thành công\n", blockID, peer.ID())
			}
		}
	}()

	// Goroutine nhận tin nhắn
	go func() {
		for {
			log.Printf("==--------------------Xử lý các yêu cầu--------------------==")

			msg, err := rw.ReadMsg()
			if err != nil {
				errChan <- fmt.Errorf("lỗi khi đọc tin nhắn từ %v: %v", peer.ID(), err)
				continue
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
					continue
				}
				if err := p2p.Send(rw, BlockResponseMsg, block); err != nil {
					log.Printf("Lỗi khi gửi block đến %v: %v\n", peer.ID(), err)
					continue
				}

			case LastBlockRequestMsg:
				lastBlock, err := p.blockchain.GetLastBlock()
				if err != nil {
					log.Printf("Lỗi khi lấy last block: %v\n", err)
					continue
				}
				if err := p2p.Send(rw, LastBlockResponseMsg, lastBlock); err != nil {
					log.Printf("Lỗi khi gửi last block đến %v: %v\n", peer.ID(), err)
					continue
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

			case TransactionRequestMsg:
				var txID string
				if err := msg.Decode(&txID); err != nil {
					log.Printf("Lỗi khi giải mã yêu cầu transaction: %v\n", err)
					continue
				}

				// Tìm transaction sử dụng GetTransactionByIDFromNode
				tx, err := p.blockchain.GetTransactionByIDFromNode(txID)
				if err != nil {
					log.Printf("Không tìm thấy transaction %s: %v\n", txID, err)
					continue
				}

				// Gửi transaction về cho peer yêu cầu
				if err := p2p.Send(rw, TransactionResponseMsg, tx); err != nil {
					log.Printf("Lỗi khi gửi transaction response: %v\n", err)
					continue
				}

			}
		}

	}()

	return <-errChan
}

// Thêm các hàm xử lý mới
func (p *P2PNetwork) handleBlockResponse(peer *p2p.Peer, block blockchain.Block) {
	log.Printf("Nhận được block từ %v:\n", peer.ID())
	p.logBlockInfo(block)

	err := p.blockchain.HandleNewBlock(block)
	if err != nil {
		log.Printf("Lỗi khi xử lý block mới: %v\n", err)
	}
}

func (p *P2PNetwork) handleLastBlockResponse(peer *p2p.Peer, block blockchain.Block) {
	log.Printf("Nhận được last block từ %v, index: %d\n", peer.ID(), block.Index)
	p.logBlockInfo(block)
	lastBlock, err := p.blockchain.GetLastBlock()
	if err != nil {
		log.Printf("Lỗi khi lấy last block: %v\n", err)
		return
	}
	if block.Index > lastBlock.Index {
		fmt.Printf("Gửi last block đến %v, index: %d\n", peer.ID(), block.Index)
		if err := p.blockchain.SaveLastBlock(block); err != nil {
			log.Printf("Lỗi khi lưu last block: %v\n", err)
			return
		}
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
	log.Printf("Nhận được yêu cầu đề xuất block t %v, index: %d\n", peer.ID(), block.Index)
	p.logBlockInfo(block)

	err := p.blockchain.HandleNewBlock(block)
	if err != nil {
		log.Printf("Lỗi khi xử lý block mới: %v\n", err)
	}

}

// BroadcastTransaction phát sóng một giao dịch đến tất cả các peer đã kết nối
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

	// Thêm xử lý lỗi để không đóng kết nối
	if len(p.txChannels) == 0 {
		log.Printf("Không có peer nào để gửi transaction")
		return fmt.Errorf("không có peer nào để gửi transaction")
	}

	return nil
}

// RequestTransaction yêu cầu một giao dịch từ các peer
func (p *P2PNetwork) RequestTransaction(txID string) (*blockchain.Transaction, error) {
	// Kiểm tra các điều kiện đầu vào
	if txID == "" {
		return nil, fmt.Errorf("transaction ID khng được để trống")
	}

	responseChan := make(chan *blockchain.Transaction)
	errorChan := make(chan error)
	var wg sync.WaitGroup

	// Lấy danh sách peer RWs
	peerRWs := p.GetAllPeerRWs()
	if len(peerRWs) == 0 {
		return nil, fmt.Errorf("không có peer nào đang kết nối")
	}

	// Broadcast request đến tất cả peers
	for peerID, rw := range peerRWs {
		// Kiểm tra rw không nil
		if rw == nil {
			log.Printf("Bỏ qua peer %s do rw là nil", peerID)
			continue
		}

		wg.Add(1)
		go func(peerID string, rw p2p.MsgReadWriter) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errorChan <- fmt.Errorf("panic khi xử lý peer %s: %v", peerID, r)
				}
			}()

			// Gửi yêu cầu transaction với timeout
			sendCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				done <- p2p.Send(rw, TransactionRequestMsg, txID)
			}()

			select {
			case err := <-done:
				if err != nil {
					if err.Error() == "shutting down" {
						log.Printf("Peer %s đang đóng kết nối", peerID)
						return
					}
					errorChan <- fmt.Errorf("lỗi khi gửi yêu cầu tới peer %s: %v", peerID, err)
					return
				}
			case <-sendCtx.Done():
				errorChan <- fmt.Errorf("timeout khi gửi yêu cầu tới peer %s", peerID)
				return
			}

			// Đọc phản hồi với timeout
			readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			msgChan := make(chan p2p.Msg, 1)
			errChan := make(chan error, 1)

			go func() {
				msg, err := rw.ReadMsg()
				if err != nil {
					errChan <- err
					return
				}
				msgChan <- msg
			}()

			select {
			case msg := <-msgChan:
				if msg.Code == TransactionResponseMsg {
					var tx blockchain.Transaction
					if err := msg.Decode(&tx); err != nil {
						errorChan <- fmt.Errorf("lỗi khi giải mã transaction từ peer %s: %v", peerID, err)
						return
					}
					responseChan <- &tx
				}
			case err := <-errChan:
				errorChan <- fmt.Errorf("lỗi khi đọc phản hồi từ peer %s: %v", peerID, err)
			case <-readCtx.Done():
				errorChan <- fmt.Errorf("timeout khi đọc phản hồi từ peer %s", peerID)
			}
		}(peerID, rw)
	}

	// Chờ phản hồi với timeout tổng thể
	select {
	case tx := <-responseChan:
		return tx, nil
	case err := <-errorChan:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout khi chờ phản hồi từ tất cả peers")
	}
}

// Lưu MsgReadWriter cho một peer
func (p *P2PNetwork) setPeerRW(peerID string, rw p2p.MsgReadWriter) {
	p.peerRWMutex.Lock()
	defer p.peerRWMutex.Unlock()
	p.peerRWs[peerID] = rw
}

// Xóa MsgReadWriter khi peer ngắt kết nối
func (p *P2PNetwork) removePeerRW(peerID string) {
	p.peerRWMutex.Lock()
	defer p.peerRWMutex.Unlock()
	delete(p.peerRWs, peerID)
}

// GetAllPeerRWs trả về map của tất cả peer MsgReadWriters
func (p *P2PNetwork) GetAllPeerRWs() map[string]p2p.MsgReadWriter {
	p.peerRWMutex.RLock()
	defer p.peerRWMutex.RUnlock()

	// Tạo bản sao của map để tránh race condition
	rwsCopy := make(map[string]p2p.MsgReadWriter)
	for id, rw := range p.peerRWs {
		rwsCopy[id] = rw
	}
	return rwsCopy
}

// Thêm hàm mới để kiểm tra và làm sạch peerRWs không hợp lệ
func (p *P2PNetwork) cleanInvalidPeerRWs() {
	p.peerRWMutex.Lock()
	defer p.peerRWMutex.Unlock()

	// Lấy danh sách peers hiện tại từ Server
	activePeers := make(map[string]bool)
	for _, peer := range p.Server.Peers() {
		activePeers[peer.ID().String()] = true
	}

	// Xóa các peerRW không còn active
	for peerID := range p.peerRWs {
		if !activePeers[peerID] {
			delete(p.peerRWs, peerID)
			log.Printf("Đã xóa peerRW không hợp lệ cho peer %s", peerID)
		}
	}
}

// Cập nhật maintainConnections để định kỳ làm sạch peerRWs
func (p *P2PNetwork) maintainConnections(nodes []*enode.Node) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Làm sạch peerRWs không hợp lệ
		p.cleanInvalidPeerRWs()

		// Thêm lại các node
		for _, node := range nodes {
			p.Server.AddPeer(node)
		}
	}
}
