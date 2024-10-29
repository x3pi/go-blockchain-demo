Dưới đây là tài liệu tiếng Việt cho các ví dụ trong thư mục `bls-demo`:

## Ví dụ 1: Chữ ký đơn (Single Signature)

### Mô tả

Ví dụ này minh họa cách tạo một chữ ký BLS đơn giản, in ra khóa riêng tư và khóa công khai, ký một thông điệp và xác minh chữ ký.

### Mã nguồn


```1:47:bls-demo/single-signature/single-signature.go
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
```


## Ví dụ 2: Khôi phục từ khóa riêng tư (Recover from Private Key)

### Mô tả

Ví dụ này hướng dẫn cách khôi phục một signer từ khóa riêng tư đã cho dưới dạng chuỗi hex và in ra khóa công khai tương ứng.

### Mã nguồn


```1:46:bls-demo/recover/recover.go
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
```


## Ví dụ 3: Chữ ký đa người ký (Multi-Signature)

### Mô tả

Ví dụ này minh họa cách tạo chữ ký đa người ký, tổng hợp chữ ký từ nhiều signer và xác minh chữ ký tổng hợp.

### Mã nguồn


```1:83:bls-demo/multi-signature/multi-signature.go
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
```


## Ví dụ 4: Chữ ký vòng (Ring Signature)

### Mô tả

Ví dụ này minh họa cách tạo và xác minh chữ ký vòng, cho phép một thành viên trong nhóm ký mà không tiết lộ danh tính của mình. Đây là phiên bản đơn đơn giản tốt nhất nên sử dụng ví dụ 5 với ngưỡng là 1 sẽ đạt được mục đích này.

### Mã nguồn


```1:140:bls-demo/ring-signatures/ring-signatures.go
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
```

## Ví dụ 5: Chữ ký ngưỡng (Threshold Signature)

### Mô tả

Ví dụ này minh họa cách tạo chữ ký ngưỡng, yêu cầu một số lượng tối thiểu chữ ký để khôi phục chữ ký hợp lệ.

### Mã nguồn


```1:125:bls-demo/threshold-signatures/threshold-signatures.go
package main

import (
	"fmt"

	"go.dedis.ch/dela/crypto"
	"go.dedis.ch/dela/crypto/bls"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/tbls"
)

type ThresholdSigner struct {
	suite     *pairing.SuiteBn256
	private   *share.PriShare
	public    *share.PubPoly
	threshold int
	total     int
}

func NewThresholdScheme(t, n int) ([]*ThresholdSigner, error) {
	if t > n {
		return nil, fmt.Errorf("ngưỡng không thể lớn hơn tổng số người tham gia")
	}

	suite := pairing.NewSuiteBn256()
	secret := suite.G2().Scalar().Pick(suite.RandomStream())
	priPoly := share.NewPriPoly(suite.G2(), t, secret, suite.RandomStream())
	pubPoly := priPoly.Commit(suite.G2().Point().Base())

	shares := priPoly.Shares(n)
	signers := make([]*ThresholdSigner, n)

	for i := 0; i < n; i++ {
		signers[i] = &ThresholdSigner{
			suite:     suite,
			private:   shares[i],
			public:    pubPoly,
			threshold: t,
			total:     n,
		}
	}

	return signers, nil
}

func (ts *ThresholdSigner) Sign(msg []byte) (crypto.Signature, error) {
	sig, err := tbls.Sign(ts.suite, ts.private, msg)
	if err != nil {
		return nil, fmt.Errorf("không thể tạo chữ ký: %v", err)
	}

	return bls.NewSignature(sig), nil
}

func RecoverSignature(msg []byte, sigs []crypto.Signature, t, n int, pubPoly *share.PubPoly) (crypto.Signature, error) {
	if len(sigs) < t {
		return nil, fmt.Errorf("không đủ chữ ký: đã có %d, cần %d", len(sigs), t)
	}

	// Chuyển đổi chữ ký thành byte
	sigBytes := make([][]byte, len(sigs))
	for i, sig := range sigs {
		bytes, err := sig.(bls.Signature).MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("không thể chuyển đổi chữ ký: %v", err)
		}
		sigBytes[i] = bytes
	}

	suite := pairing.NewSuiteBn256()

	// Khôi phục chữ ký
	recovered, err := tbls.Recover(suite, pubPoly, msg, sigBytes, t, n)
	if err != nil {
		return nil, fmt.Errorf("không thể khôi phục chữ ký: %v", err)
	}

	return bls.NewSignature(recovered), nil
}

func main() {
	t := 1 // ngưỡng
	n := 3 // tổng số người tham gia

	signers, err := NewThresholdScheme(t, n)
	if err != nil {
		panic(err)
	}

	message := []byte("Test threshold signature")

	// Thu thập t chữ ký
	signatures := make([]crypto.Signature, t)
	for i := 0; i < t; i++ {
		sig, err := signers[i].Sign(message)
		if err != nil {
			panic(fmt.Sprintf("Ký thất bại: %v", err))
		}
		signatures[i] = sig
	}

	// Khôi phục chữ ký ngưỡng
	recovered, err := RecoverSignature(message, signatures, t, n, signers[0].public)
	if err != nil {
		panic(fmt.Sprintf("Khôi phục thất bại: %v", err))
	}

	// Tạo trình xác minh sử dụng khóa công khai của nhóm
	verifier := bls.NewSigner().GetVerifierFactory()
	v, err := verifier.FromArray([]crypto.PublicKey{
		bls.NewPublicKeyFromPoint(signers[0].public.Commit()),
	})
	if err != nil {
		panic(fmt.Sprintf("Tạo trình xác minh thất bại: %v", err))
	}

	// Xác minh chữ ký đã khôi phục
	if err := v.Verify(message, recovered); err != nil {
		fmt.Println("Xác minh chữ ký thất bại:", err)
	} else {
		fmt.Println("Xác minh chữ ký ngưỡng thành công!")
		fmt.Printf("Số chữ ký cần thiết: %d trên tổng số %d\n", t, n)
	}
}
```

Hy vọng tài liệu này sẽ giúp hiểu rõ hơn về cách sử dụng chữ ký BLS trong các tình huống khác nhau.