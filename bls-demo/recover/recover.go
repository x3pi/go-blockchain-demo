package main

import (
	"encoding/hex"
	"fmt"

	"go.dedis.ch/dela/crypto/bls"
)

func main() {
	// Giả sử bạn đã có private key dưới dạng hex string
	privateKeyHex := "7ba018e438a0343d86e50ada05578624e6db846581e79ee82af41323e28712f4" // Thay thế bằng private key thực tế của bạn

	// Chuyển đổi hex string thành bytes
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		panic(fmt.Sprintf("Không thể chuyển đổi hex thành bytes: %v", err))
	}

	// Khôi phục signer từ private key
	signer, err := bls.NewSignerFromBytes(privateKeyBytes)
	if err != nil {
		panic(fmt.Sprintf("Không thể khôi phục signer: %v", err))
	}

	// In ra public key để xác nhận
	publicKey := signer.GetPublicKey()
	publicKeyBytes, err := publicKey.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Không thể lấy public key: %v", err))
	}

	fmt.Printf("Public key (hex): %x\n", publicKeyBytes)
	//Public key (hex): 0404cf859bac254ceb08acf4a3597462058d471554dac773d0fd589e87482fe787b7352c2a8c4570bea82c3de815167622904601b79aafa23a9b37ac4db352167040b2d6601ed1d629d5476afa99e7bdfbbb218949e7c18f89a8f3499a1c23bd43e4aaeb034fc779bac75f69f39d3b537b26ac5401829420aef64d3f81995634

	// Một số cặp ví dụ khác

	// Private key (hex): 5e6aa2c4c487915f10addde02e5ac839c1cba9380ec903e36faab18f91234555
	// Public key (hex): 5df9fb8afcc9d1db1a4f5da33a2a748a727d6313d8200e904e41bf0dcc78bbb06104ccf73438576e3848f1b356fa61384594c3f1b6002efdbd0ee49bf78cab56865ab5bbf48028db470adf6e7817bec310863652f216d57894a98c903615f90f09e328427cb4fa32da1bdb17f55e18cf34d4362dadf0ad413b9a1652d2b7d6e2

	// Private key (hex): 1b684c9933dacbfb3b363ec14e097476ee1c52c67a637028810f490abadbb0e7
	// Public key (hex): 0923227dda8d53fa1434fd51bc74281f181aeed280fe58564cc2df120df6a8820dadc0994e48b5c965157c914e4c9e3424b572a6098d0679247dfc1921d2154030080f9e42e06572ba33c140cec53b8c01069d36fa881a6991c53cf69358341b7925e756249d645561999e1534a86e2d59eda55772247f8caa19988728bea4d6

	// Private key (hex): 47f626cec2d259a9e4fbf516c92c0b6537b2603495749300e87263e944a7532c
	// Public key (hex): 522cf285df7711240972469c653c899b197dc884b23f0013c7b99d72bbf3b2d284d5175e6cb6eaa3690d9d4278fd6e3f92f6134c8d392a5ccadb2f4abce1b3f56f35fe878c5dc94be925e800955f7f084fc1db34183a5fd83e28819ffa1cb0f765f247d9feb1fa6703c9f773658fa5d577931a0d1b13d62d06b88742a7ff4b86
}
