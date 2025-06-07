package generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type CodeGenerator struct{}

func NewCodeGenerator() *CodeGenerator {
	return &CodeGenerator{}
}

func (g *CodeGenerator) GenerateCheckoutCode(saleID, userID string) (string, error) {
	randomBytes := make([]byte, 8)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	randomHex := hex.EncodeToString(randomBytes)

	return fmt.Sprintf("CHK-%s-%s", saleID, randomHex), nil
}

func (g *CodeGenerator) GenerateSaleID() string {
	randomBytes := make([]byte, 5) // 5 bytes will give us 10 hex chars
	if _, err := rand.Read(randomBytes); err != nil {
		return ""
	}
	randomId := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("S-%s", randomId)
}

func (g *CodeGenerator) GenerateCheckoutID() string {
	randomBytes := make([]byte, 5) // 5 bytes will give us 10 hex chars
	if _, err := rand.Read(randomBytes); err != nil {
		return ""
	}
	randomId := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("C-%s", randomId)
}
