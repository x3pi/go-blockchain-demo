package main

import (
	"encoding/hex"
	"fmt"

	"go.dedis.ch/dela/crypto/bls"
)

func main() {
	// Tạo một signer mới
	signer := bls.NewSigner()

	// In private key
	privateKeyBytes, err := signer.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Không thể lấy private key: %v", err))
	}
	fmt.Printf("Private key (hex): %s\n", hex.EncodeToString(privateKeyBytes))

	// Message cần ký
	message := []byte("Hello BLS Signature")

	// Ký message
	signature, err := signer.Sign(message)
	if err != nil {
		panic(fmt.Sprintf("Không thể ký: %v", err))
	}

	// Lấy public key từ signer
	publicKey := signer.GetPublicKey()

	// In public key
	publicKeyBytes, err := publicKey.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Không thể lấy public key: %v", err))
	}
	fmt.Printf("Public key (hex): %s\n", hex.EncodeToString(publicKeyBytes))

	// Xác minh chữ ký
	err = publicKey.Verify(message, signature)
	if err != nil {
		panic(fmt.Sprintf("Xác minh chữ ký thất bại: %v", err))
	}

	fmt.Println("Xác minh chữ ký thành công!")
}
