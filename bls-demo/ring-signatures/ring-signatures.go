package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"

	"go.dedis.ch/dela/crypto"
	"go.dedis.ch/dela/crypto/bls"
	"go.dedis.ch/kyber/v3/pairing"
)

// RingSignature đại diện cho chữ ký vòng
type RingSignature struct {
	C       []byte             // Challenge
	S       [][]byte           // Responses cho mỗi thành viên
	Message []byte             // Message được ký
	Ring    []crypto.PublicKey // Danh sách public keys trong vòng
}

func main() {
	// Tạo một nhóm gồm 5 người dùng
	ringSize := 5
	signers := make([]bls.Signer, ringSize)
	ring := make([]crypto.PublicKey, ringSize)

	// Khởi tạo các cặp khóa cho mỗi thành viên
	for i := 0; i < ringSize; i++ {
		signers[i] = bls.NewSigner()
		ring[i] = signers[i].GetPublicKey()
	}

	// Chọn một người ký ngẫu nhiên
	max := big.NewInt(int64(ringSize))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		fmt.Printf("Lỗi khi tạo chỉ số ngẫu nhiên: %v\n", err)
		return
	}
	actualSignerIndex := int(n.Int64())

	message := []byte("Đây là một ví dụ về chữ ký vòng")

	// Tạo ring signature
	ringSignature, err := createRingSignature(signers[actualSignerIndex], ring, actualSignerIndex, message)
	if err != nil {
		fmt.Printf("Lỗi khi tạo chữ ký vòng: %v\n", err)
		return
	}

	// Xác thực ring signature
	valid := verifyRingSignature(ring, message, ringSignature)

	if valid {
		fmt.Println("Xác thực chữ ký vòng thành công!")
		fmt.Printf("Chữ ký có thể được tạo bởi bất kỳ thành viên nào trong số %d\n", ringSize)
		fmt.Printf("Người ký thực sự là thành viên %d, nhưng điều này được ẩn khỏi người xác thực\n", actualSignerIndex)
	} else {
		fmt.Println("Xác thực chữ ký vòng thất bại!")
	}
}

func createRingSignature(signer bls.Signer, ring []crypto.PublicKey,
	signerIndex int, message []byte) (*RingSignature, error) {

	ringSize := len(ring)
	responses := make([][]byte, ringSize)

	// Tạo responses ngẫu nhiên cho các thành viên không phải người ký
	for i := 0; i < ringSize; i++ {
		if i != signerIndex {
			tempSigner := bls.NewSigner()
			sig, err := tempSigner.Sign(message)
			if err != nil {
				return nil, fmt.Errorf("không thể tạo chữ ký ngẫu nhiên: %v", err)
			}
			responses[i], err = sig.MarshalBinary()
			if err != nil {
				return nil, fmt.Errorf("không thể tạo chữ ký: %v", err)
			}
		}
	}

	// Tạo chữ ký thực cho người ký
	sig, err := signer.Sign(message)
	if err != nil {
		return nil, fmt.Errorf("không thể tạo chữ ký: %v", err)
	}
	responses[signerIndex], err = sig.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("không thể tạo chữ ký: %v", err)
	}

	// Tạo challenge bằng cách hash tất cả responses
	hasher := pairing.NewSuiteBn256().Hash()
	for i := 0; i < ringSize; i++ {
		hasher.Write(responses[i])
	}
	challenge := hasher.Sum(nil)

	return &RingSignature{
		C:       challenge,
		S:       responses,
		Message: message,
		Ring:    ring,
	}, nil
}

func verifyRingSignature(ring []crypto.PublicKey, message []byte,
	signature *RingSignature) bool {

	if len(signature.S) != len(ring) {
		return false
	}

	// Verify từng response
	validSigFound := false
	for i := 0; i < len(ring); i++ {
		sig := bls.NewSignature(signature.S[i])
		err := ring[i].Verify(message, sig)
		if err == nil {
			validSigFound = true
			break
		}
	}

	if !validSigFound {
		return false
	}

	// Verify challenge
	hasher := pairing.NewSuiteBn256().Hash()
	for i := 0; i < len(ring); i++ {
		hasher.Write(signature.S[i])
	}
	computedChallenge := hasher.Sum(nil)

	return bytes.Equal(signature.C, computedChallenge)
}
