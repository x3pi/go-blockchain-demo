package p2pnetwork

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/rand"
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

	blockProto := p2p.Protocol{
		Name:    "block",
		Version: 1,
		Length:  9,
		Run:     p.runBlockProtocol,
	}

	cfg.Protocols = []p2p.Protocol{blockProto}

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

// Hàm xử lý đề xuất block mỗi 5 giây
func (p *P2PNetwork) handleBlockProposals(peer *p2p.Peer, rw p2p.MsgReadWriter) {
	reconnectAttempts := 0
	backoff := time.Second

	for {
		time.Sleep(5 * time.Second)

		if !p.isPeerConnected(peer.ID()) {
			if err := p.handleReconnect(peer, &reconnectAttempts, &backoff, &rw); err != nil {
				continue
			}
		}

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
			}
		}
	}
}

// Hàm xử lý yêu cầu block mỗi giây
func (p *P2PNetwork) handleBlockRequests(peer *p2p.Peer, rw p2p.MsgReadWriter) {
	reconnectAttempts := 0
	for {
		time.Sleep(1 * time.Second)

		if !p.isPeerConnected(peer.ID()) {
			if err := p.handleReconnect(peer, &reconnectAttempts, nil, &rw); err != nil {
				continue
			}
		}

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

		if currentBlock.Index < lastBlock.Index {
			blockID := fmt.Sprintf("block_%d", currentBlock.Index+1)
			request := BlockRequest{
				Type:    "request",
				BlockID: blockID,
			}
			if err := p2p.Send(rw, BlockRequestMsg, request); err != nil {
				log.Printf("Lỗi khi gửi yêu cầu block: %v", err)
			} else {
				log.Printf("Đã gửi yêu cầu block %s đến %v thành công\n", blockID, peer.ID())
			}
		}
	}
}

// Hàm xử lý yêu cầu last block mỗi giây
func (p *P2PNetwork) handleLastBlockRequests(peer *p2p.Peer, rw p2p.MsgReadWriter) {
	reconnectAttempts := 0
	backoff := time.Second

	for {
		time.Sleep(1 * time.Second)

		if !p.isPeerConnected(peer.ID()) {
			if err := p.handleReconnect(peer, &reconnectAttempts, &backoff, &rw); err != nil {
				continue
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		done := make(chan error, 1)

		go func() {
			lastBlockRequest := BlockRequest{
				Type: "last_block",
			}
			done <- p2p.Send(rw, LastBlockRequestMsg, lastBlockRequest)
		}()

		select {
		case err := <-done:
			if err != nil {
				if err.Error() == "shutting down" || err.Error() == "use of closed network connection" {
					log.Printf("Kết nối với peer %v đã đóng, đang thử kết nối lại...", peer.ID())
					time.Sleep(5 * time.Second)
				} else {
					log.Printf("Lỗi khi gửi yêu cầu last block: %v", err)
				}
			} else {
				log.Printf("Đã gửi yêu cầu last block đến %v thành công", peer.ID())
			}
		case <-ctx.Done():
			log.Printf("Timeout khi gửi yêu cầu last block đến %v", peer.ID())
		}
		cancel()
	}
}

// Hàm helper để xử lý kết nối lại
func (p *P2PNetwork) handleReconnect(peer *p2p.Peer, attempts *int, backoff *time.Duration, rw *p2p.MsgReadWriter) error {
	log.Printf("Phát hiện mất kết nối với peer %v", peer.ID())

	jitter := time.Duration(rand.Int63n(1000)) * time.Millisecond
	time.Sleep(jitter)

	if err := p.reconnectToPeer(peer.Node()); err != nil {
		*attempts++
		if *attempts >= maxReconnectAttempts {
			log.Printf("Đã thử kết nối lại %d lần không thành công với peer %v",
				maxReconnectAttempts, peer.ID())
			*attempts = 0
			if backoff != nil {
				*backoff = time.Second
			}
			time.Sleep(60 * time.Second)
			return fmt.Errorf("đã đạt giới hạn số lần thử kết nối lại")
		}

		if backoff != nil {
			*backoff *= 2
			if *backoff > 30*time.Second {
				*backoff = 30 * time.Second
			}
			jitter := time.Duration(rand.Int63n(int64(*backoff)))
			actualBackoff := *backoff + jitter
			log.Printf("Kết nối lại thất bại lần %d với peer %v. Thử lại sau %v",
				*attempts, peer.ID(), actualBackoff)
			time.Sleep(actualBackoff)
		}
		return err
	}

	*attempts = 0
	if backoff != nil {
		*backoff = time.Second
	}
	log.Printf("Đã kết nối lại thành công với peer %v", peer.ID())

	if newRW := p.getPeerRW(peer.ID().String()); newRW != nil {
		*rw = newRW
	}
	return nil
}

// Cập nhật hàm runBlockProtocol để sử dụng các hàm mới
func (p *P2PNetwork) runBlockProtocol(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	// Thiết lập ban đầu
	p.setPeerRW(peer.ID().String(), rw)
	defer p.removePeerRW(peer.ID().String())

	txChan := make(chan *blockchain.Transaction, 100)
	p.registerTransactionChannel(peer.ID().String(), txChan)
	defer p.unregisterTransactionChannel(peer.ID().String())

	// Xử lý tin nhắn đến
	go p.handleIncomingBlockMessages(peer, rw)

	// Xử lý transaction
	go func() {
		for tx := range txChan {
			if err := p2p.Send(rw, TransactionMsg, tx); err != nil {
				log.Printf("Lỗi khi gửi transaction tới peer %v: %v", peer.ID(), err)
			}
		}
	}()

	// Khởi chạy 3 goroutine chính
	go p.handleBlockProposals(peer, rw)
	go p.handleBlockRequests(peer, rw)
	go p.handleLastBlockRequests(peer, rw)

	// Chờ vô hạn
	select {}
}

func (p *P2PNetwork) handleIncomingBlockMessages(peer *p2p.Peer, rw p2p.MsgReadWriter) {
	for {
		msg, err := rw.ReadMsg()
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "use of closed network connection" {
				log.Printf("Kết nối với peer %v đã đóng, đang dọn dẹp...", peer.ID())
				p.cleanupPeerConnection(peer.ID().String())
				return
			}
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
				continue
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
			}
		}

	}
}

// Thêm hàm mới để dọn dẹp kết nối peer
func (p *P2PNetwork) cleanupPeerConnection(peerID string) {
	p.peerRWMutex.Lock()
	if _, exists := p.peerRWs[peerID]; exists {
		delete(p.peerRWs, peerID)
		log.Printf("Đã xóa peerRW cho %s", peerID)
	}
	p.peerRWMutex.Unlock()

	p.txChannelMutex.Lock()
	if ch, exists := p.txChannels[peerID]; exists {
		close(ch)
		delete(p.txChannels, peerID)
		log.Printf("Đã đóng và xóa kênh giao dịch cho %s", peerID)
	}
	p.txChannelMutex.Unlock()

	// Đảm bảo xóa khỏi danh sách peers của server
	if p.Server != nil {
		for _, peer := range p.Server.Peers() {
			if peer.ID().String() == peerID {
				p.Server.RemovePeer(peer.Node())
				log.Printf("Đã xóa peer %s khỏi server", peerID)
				break
			}
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
	maxReconnectAttempts  = 5
	reconnectDelay        = 10 * time.Second
	initialReconnectDelay = 5 * time.Second
	maxReconnectDelay     = 2 * time.Minute
	reconnectJitterRange  = 1000 // milliseconds
	healthCheckInterval   = 30 * time.Second
)

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
	log.Printf("Nhận đưc last block từ %v, index: %d\n", peer.ID(), block.Index)
	p.logBlockInfo(block)
	lastBlock, err := p.blockchain.GetLastBlock()
	if err != nil {
		log.Printf("Lỗi khi lấy last block: %v\n", err)
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

	// Thêm xử lý lỗi để không đóng kết nối
	if len(p.txChannels) == 0 {
		log.Printf("Không có peer nào để gửi transaction")
		return fmt.Errorf("không có peer nào để gửi transaction")
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
				log.Printf("Đã cập nh��t peerRW cho peer %v sau khi kết nối lại", peer.ID())
			}
			return nil // Kết nối thành cng
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

// Thêm hàm mới để duy trì kết nối
func (p *P2PNetwork) maintainConnections(nodes []*enode.Node) {
	reconnectDelay := initialReconnectDelay
	healthCheckTicker := time.NewTicker(healthCheckInterval)
	defer healthCheckTicker.Stop()

	// Add initial connection attempts with retry
	for i := 0; i < 5; i++ {
		if len(p.Server.Peers()) > 0 {
			break
		}
		log.Printf("Attempting initial connections, attempt %d/5", i+1)
		for _, node := range nodes {
			p.Server.AddPeer(node)
		}
		time.Sleep(5 * time.Second)
	}

	for {
		select {
		case <-healthCheckTicker.C:
			currentPeers := p.Server.Peers()
			if len(currentPeers) == 0 {
				log.Println("No peers connected, attempting to restart P2P...")

				if err := p.RestartP2PConnection(); err != nil {
					log.Printf("Error restarting P2P: %v", err)
					reconnectDelay *= 2
					if reconnectDelay > maxReconnectDelay {
						reconnectDelay = maxReconnectDelay
					}
					time.Sleep(reconnectDelay)
					continue
				}
				reconnectDelay = initialReconnectDelay
			}

			// Check and reconnect to disconnected nodes
			for _, node := range nodes {
				if !p.isPeerConnected(node.ID()) {
					go p.reconnectWithBackoffAndRetry(node)
				}
			}
		}
	}
}

// Thêm hàm mới để xử lý kết nối lại với retry và backoff
func (p *P2PNetwork) reconnectWithBackoffAndRetry(node *enode.Node) {
	backoff := initialReconnectDelay
	maxAttempts := 10
	attempt := 0

	for attempt < maxAttempts {
		log.Printf("Đang thử kết nối lại với node %v (lần thử %d/%d)", node.ID(), attempt+1, maxAttempts)

		// Xóa peer cũ và cleanup
		p.Server.RemovePeer(node)
		p.cleanupPeerConnection(node.ID().String())
		time.Sleep(2 * time.Second)

		// Thêm jitter
		jitter := time.Duration(rand.Int63n(reconnectJitterRange)) * time.Millisecond
		actualBackoff := backoff + jitter

		// Thử kết nối lại
		p.Server.AddPeer(node)

		// Đợi và kiểm tra kết nối
		time.Sleep(actualBackoff)

		if p.isPeerConnected(node.ID()) {
			log.Printf("Đã kết nối lại thành công với node %v", node.ID())
			return
		}

		attempt++
		if attempt < maxAttempts {
			backoff *= 2
			if backoff > maxReconnectDelay {
				backoff = maxReconnectDelay
			}
			log.Printf("Kết nối lại thất bại, đợi %v trước lần thử tiếp theo", backoff)
		}
	}

	log.Printf("Không thể kết nối lại với node %v sau %d lần thử", node.ID(), maxAttempts)
}

// Thêm hàm mới để khởi động lại kết nối P2P
func (p *P2PNetwork) RestartP2PConnection() error {
	log.Println("Đang khởi động lại kết nối P2P...")

	// Dừng server hiện tại nếu đang chạy
	if p.Server != nil {
		p.Server.Stop()
		time.Sleep(5 * time.Second) // Đợi server dừng hoàn toàn
	}

	// Khởi tạo lại các map
	p.peerRWMutex.Lock()
	p.peerRWs = make(map[string]p2p.MsgReadWriter)
	p.peerRWMutex.Unlock()

	p.txChannelMutex.Lock()
	p.txChannels = make(map[string]chan *blockchain.Transaction)
	p.txChannelMutex.Unlock()

	// Khởi động lại server với cấu hình mới
	return p.Start()
}
