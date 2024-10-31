package p2pnetwork

import (
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
		Length:  9,
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
	// Lưu rw cho peer này
	p.setPeerRW(peer.ID().String(), rw)
	defer p.removePeerRW(peer.ID().String())

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
	// Goroutine 1: Chạy mỗi 5 giây
	go func() {
		reconnectAttempts := 0
		for {
			// Kiểm tra kết nối
			if !p.isPeerConnected(peer.ID()) {
				log.Printf("Phát hiện peer %v đã ngắt kết nối", peer.ID())

				// Thử kết nối lại
				if err := p.reconnectToPeer(peer.Node()); err != nil {
					reconnectAttempts++
					if reconnectAttempts >= maxReconnectAttempts {
						log.Printf("Đã thử kết nối lại %d lần không thành công với peer %v, tiếp tục vòng lặp",
							maxReconnectAttempts, peer.ID())
						reconnectAttempts = 0 // Reset số lần thử
						continue              // Tiếp tục vòng lặp thay vì return
					}
					log.Printf("Kết nối lại thất bại lần %d với peer %v: %v. Thử lại sau %v",
						reconnectAttempts, peer.ID(), err, reconnectDelay)
					time.Sleep(reconnectDelay)
					continue
				}

				// Kết nối lại thành công
				reconnectAttempts = 0
				log.Printf("Đã kết nối lại thành công với peer %v", peer.ID())

				// Khởi tạo lại rw mới cho peer
				if newRW := p.getPeerRW(peer.ID().String()); newRW != nil {
					rw = newRW
				}
			}

			now := time.Now()
			nextRun := now.Truncate(5 * time.Second).Add(5 * time.Second)
			if nextRun.Before(now) {
				nextRun = nextRun.Add(5 * time.Second)
			}
			time.Sleep(nextRun.Sub(now))

			// Gửi yêu cầu last block với xử lý lỗi tốt hơn
			lastBlockRequest := BlockRequest{
				Type: "last_block",
			}
			if err := p2p.Send(rw, LastBlockRequestMsg, lastBlockRequest); err != nil {
				if err.Error() == "shutting down" {
					log.Printf("Kết nối với peer %v đã đóng", peer.ID())
					continue
				}
				log.Printf("Lỗi khi gửi yêu cầu last block: %v", err)
				continue
			}
			log.Printf("Đã gửi yêu cầu last block đến %v thành công\n", peer.ID())

			// Gửi yêu cầu đề xuất khối
			p.blockchain.Mempool.PrintMempool()
			block, err := p.blockchain.ProposeNewBlock()
			if err != nil {
				log.Printf("Lỗi khi đề xuất block: %v\n", err)
				continue // Tiếp tục vòng lặp thay vì dừng lại
			}

			// Thêm kiểm tra block index > 0
			if block.Index > 0 {
				fmt.Printf("Đề xuất khối index: %d\n", block.Index)
				fmt.Printf("Đề xuất khối: %+v\n", block)
				if err := p2p.Send(rw, BlockProposalRequestMsg, block); err != nil {
					log.Printf("Lỗi khi gửi đề xuất block đến %v: %v\n", peer.ID(), err)
				}
			}
		}
	}()

	// Goroutine 2: Chức năng mới lặp 1 giây
	go func() {
		for {
			// fmt.Println("Goroutine 2: Chạy mỗi 1 giây")
			time.Sleep(1 * time.Second)

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

			// Gửi yêu cầu lấy block
			if currentBlock.Index < lastBlock.Index {
				blockID := fmt.Sprintf("block_%d", currentBlock.Index+1)
				request := BlockRequest{
					Type:    "request",
					BlockID: blockID,
				}
				if err := p2p.Send(rw, BlockRequestMsg, request); err != nil {
					log.Printf("Lỗi khi gửi yêu cầu block: %v", err)
					continue
				} else {
					log.Printf("Đã gửi yêu cầu block %s đến %v thành công\n", blockID, peer.ID())
				}
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
	TransactionRequestMsg
	TransactionResponseMsg
	// Thêm hằng số cho retry
	maxReconnectAttempts = 5
	reconnectDelay       = 10 * time.Second
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
	log.Printf("Nhận được last block từ %v, index: %d\n", peer.ID(), block.Index)
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
	log.Printf("Nhận được yêu cầu đề xuất block từ %v, index: %d\n", peer.ID(), block.Index)
	p.logBlockInfo(block)

	err := p.blockchain.HandleNewBlock(block)
	if err != nil {
		log.Printf("Lỗi khi xử lý block mới: %v\n", err)
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

// RequestTransaction requests a transaction from peers
func (p *P2PNetwork) RequestTransaction(txID string) (*blockchain.Transaction, error) {
	responseChan := make(chan *blockchain.Transaction)
	errorChan := make(chan error)

	// Sử dụng GetAllPeerRWs để lấy map của tất cả peer RWs
	peerRWs := p.GetAllPeerRWs()

	// Broadcast request đến tất cả peers
	for peerID, rw := range peerRWs {
		go func(peerID string, rw p2p.MsgReadWriter) {
			// Gửi yêu cầu transaction
			fmt.Printf("Gửi yêu cầu transaction tới peer %s\n xxxxxxxxxxxxxxxxxxxxxxxxxxxxx", peerID)
			if err := p2p.Send(rw, TransactionRequestMsg, txID); err != nil {
				errorChan <- fmt.Errorf("lỗi khi gửi yêu cầu transaction tới peer %s: %v", peerID, err)
				return
			}

			// Đọc phản hồi
			msg, err := rw.ReadMsg()
			if err != nil {
				errorChan <- fmt.Errorf("lỗi khi đọc phản hồi từ peer %s: %v", peerID, err)
				return
			}

			if msg.Code == TransactionResponseMsg {
				var tx blockchain.Transaction
				if err := msg.Decode(&tx); err != nil {
					errorChan <- fmt.Errorf("lỗi khi giải mã transaction từ peer %s: %v", peerID, err)
					return
				}
				responseChan <- &tx
			}
		}(peerID, rw)
	}

	// Chờ phản hồi đầu tiên thành công
	select {
	case tx := <-responseChan:
		return tx, nil
	case err := <-errorChan:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout khi chờ phản hồi từ peers")
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

// Thêm hàm mới để xử lý kết nối lại
func (p *P2PNetwork) reconnectToPeer(node *enode.Node) error {
	// Thêm peer vào danh sách static peers
	p.Server.AddPeer(node)

	// Đợi một khoảng thời gian ngắn để kết nối được thiết lập
	time.Sleep(2 * time.Second)

	// Kiểm tra xem peer đã được kết nối chưa
	peers := p.Server.Peers()
	for _, peer := range peers {
		if peer.Node().ID() == node.ID() {
			// Cập nhật lại peerRWs cho peer mới kết nối
			if rw := p.getPeerRW(peer.ID().String()); rw != nil {
				p.setPeerRW(peer.ID().String(), rw)
				log.Printf("Đã cập nhật peerRW cho peer %v sau khi kết nối lại", peer.ID())
			}
			return nil // Kết nối thành công
		}
	}

	return fmt.Errorf("không thể kết nối lại với peer")
}

// Thêm hàm tiện ích để kiểm tra trạng thái kết nối
func (p *P2PNetwork) isPeerConnected(nodeID enode.ID) bool {
	peers := p.Server.Peers()
	for _, peer := range peers {
		if peer.Node().ID() == nodeID {
			return true
		}
	}
	return false
}

// Thêm hàm helper mới
func (p *P2PNetwork) getPeerRW(peerID string) p2p.MsgReadWriter {
	p.peerRWMutex.RLock()
	defer p.peerRWMutex.RUnlock()
	return p.peerRWs[peerID]
}
