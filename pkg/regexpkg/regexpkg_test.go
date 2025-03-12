package regexpkg

import (
	"testing"
)

func TestExtractHash(t *testing.T) {
	hash, err := ExtractHash("trxDPAXQHHPong-120325-16:04:41 296477")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if hash != "trxDPAXQHHPong" {
		t.Errorf("expected hash to be trxDPAXQHHPong, got %v", hash)
	}

	hash, err = ExtractHash("BILLTRXDPAXQHHPONG")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if hash != "TRXDPAXQHHPONG" {
		t.Errorf("expected hash to be TRXDPAXQHHPONG, got %v", hash)
	}

	hash, err = ExtractHash("BILL trxDPAXQhhPong")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if hash != "trxDPAXQhhPong" {
		t.Errorf("expected hash to be trxDPAXQhhPong, got %v", hash)
	}
}
