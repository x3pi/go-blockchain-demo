package main

import (
	"fmt"

	"go.dedis.ch/dela/crypto"
	"go.dedis.ch/dela/crypto/bls"
)

func main() {
	// Tạo 4 signer
	signer1 := bls.NewSigner()
	signer2 := bls.NewSigner()
	signer3 := bls.NewSigner()

	// Tạo signer 4 để verify khác mấy signer trên
	signer4 := bls.NewSigner()

	// Thu thập public key của tất cả người ký
	publicKeys := []crypto.PublicKey{
		signer1.GetPublicKey(),
		signer2.GetPublicKey(),
		signer3.GetPublicKey(),
	}

	// Thông điệp cần ký
	message := []byte("Đây là thông điệp quan trọng cần được ký bởi nhiều người")

	// Mỗi người ký sẽ ký thông điệp
	sig1, err := signer1.Sign(message)
	if err != nil {
		panic("Lỗi khi ký với signer 1: " + err.Error())
	}

	sig2, err := signer2.Sign(message)
	if err != nil {
		panic("Lỗi khi ký với signer 2: " + err.Error())
	}

	sig3, err := signer3.Sign(message)
	if err != nil {
		panic("Lỗi khi ký với signer 3: " + err.Error())
	}

	// Tổng hợp tất cả chữ ký thành một chữ ký duy nhất
	aggregatedSig, err := signer1.Aggregate(sig1, sig2, sig3)
	if err != nil {
		panic("Lỗi khi tổng hợp chữ ký: " + err.Error())
	}

	// Tạo bộ xác thực từ danh sách public key
	verifier, err := signer1.GetVerifierFactory().FromArray(publicKeys)
	if err != nil {
		panic("Lỗi khi tạo bộ xác thực: " + err.Error())
	}

	// Xác thực chữ ký tổng hợp
	err = verifier.Verify(message, aggregatedSig)
	if err != nil {
		fmt.Println("Xác thực thất bại:", err)
	} else {
		fmt.Println("Xác thực chữ ký tổng hợp thành công!")
	}

	// Thử dùng signer4 để verify
	// Tổng hợp tất cả chữ ký thành một chữ ký duy nhất
	aggregatedSigOther, err := signer4.Aggregate(sig1, sig2, sig3)
	if err != nil {
		panic("Lỗi khi tổng hợp chữ ký bằng signer4: " + err.Error())
	}

	// Tạo bộ xác thực từ danh sách public key
	verifierOther, err := signer4.GetVerifierFactory().FromArray(publicKeys)
	if err != nil {
		panic("Lỗi khi tạo bộ xác thực bằng signer4: " + err.Error())
	}

	// Xác thực chữ ký tổng hợp
	err = verifierOther.Verify(message, aggregatedSigOther)
	if err != nil {
		fmt.Println("Xác thực bằng signer4 thất bại:", err)
	} else {
		fmt.Println("Xác thực chữ ký bằng signer4 tổng hợp thành công!")
	}
}
