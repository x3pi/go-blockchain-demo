package p2pnetwork

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type P2PNetwork struct {
	Config     *Config
	Server     *p2p.Server
	PrivateKey *ecdsa.PrivateKey
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

	cfg.Protocols = []p2p.Protocol{proto}

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
