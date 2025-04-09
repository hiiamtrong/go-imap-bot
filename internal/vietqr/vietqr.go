package vietqr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/s3"
)

const (
	BaseURL = "https://api.vietqr.io/v2/generate"
)

type VietQRService struct {
	clientID    string
	apiKey      string
	bankID      int64
	accountNo   string
	accountName string
	template    string

	s3Service *s3.S3Service
}

type GenerateQRRequest struct {
	AccountNo   string `json:"accountNo"`   // Bank account number (6-19 chars)
	AccountName string `json:"accountName"` // Account holder name
	AcqID       int64  `json:"acqId"`       // Bank ID (BIN)
	Amount      int64  `json:"amount"`      // Transfer amount
	AddInfo     string `json:"addInfo"`     // Transfer content
	Template    string `json:"template"`    // QR template type
}

type GenerateQRResponse struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
	Data struct {
		AcqID       int    `json:"acpId"`
		AccountName string `json:"accountName"`
		QRCode      string `json:"qrCode"`    // Text QR code
		QRDataURL   string `json:"qrDataURL"` // Base64 QR image
	} `json:"data"`
}

func NewVietQRService(config *config.Config, s3Service *s3.S3Service) *VietQRService {
	return &VietQRService{
		clientID:    config.VietQR.ClientID,
		apiKey:      config.VietQR.APIKey,
		bankID:      config.VietQR.BankID,
		accountNo:   config.VietQR.AccountNo,
		accountName: config.VietQR.AccountName,
		template:    config.VietQR.Template,
		s3Service:   s3Service,
	}
}

func (s *VietQRService) GenerateQR(req *GenerateQRRequest) (*GenerateQRResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set required headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-client-id", s.clientID)
	httpReq.Header.Set("x-api-key", s.apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	var result GenerateQRResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if result.Code != "00" {
		return nil, fmt.Errorf("API error: %s", result.Desc)
	}

	return &result, nil
}

// Helper function to generate QR for a split payment
func (s *VietQRService) GenerateSplitQR(amount int64, hash string) (string, error) {
	req := &GenerateQRRequest{
		AccountNo:   s.accountNo,
		AccountName: s.accountName,
		AcqID:       s.bankID,
		Amount:      amount,
		AddInfo:     fmt.Sprintf("Bill %s", hash),
		Template:    s.template,
	}

	resp, err := s.GenerateQR(req)
	if err != nil {
		return "", err
	}

	// Upload QR code to S3
	qrURL, err := s.s3Service.UploadBase64Image(resp.Data.QRDataURL, hash, "qr-code")
	if err != nil {
		return "", err
	}

	return qrURL, nil
}
