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
	t := 2 // ngưỡng
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
