package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func genRsaKey() {
	_, err := os.Stat("/opt/private_key.pem")
	if err == nil {
		return
	}
	log.Print("gen rsa keys")
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	publicKey := &privateKey.PublicKey
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateBlock := pem.Block{Type: "RSA Private Key", Bytes: privateKeyBytes}
	if err := os.WriteFile("/opt/private_key.pem", pem.EncodeToMemory(&privateBlock), 0600); err != nil {
		panic(err)
	}
	publicKeyBytes := x509.MarshalPKCS1PublicKey(publicKey)
	publicBlock := pem.Block{Type: "RSA Public Key", Bytes: publicKeyBytes}
	if err := os.WriteFile("/opt/public_key.pem", pem.EncodeToMemory(&publicBlock), 0644); err != nil {
		panic(err)
	}
}

func loadPrivateKey() (*rsa.PrivateKey, error) {
	data, err := os.ReadFile("/opt/private_key.pem")
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing the key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func loadPublicKey() (*rsa.PublicKey, error) {
	data, err := os.ReadFile("/opt/public_key.pem")
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing the key")
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return publicKey, nil
}

func getPublicKey(c *gin.Context) {
	data, err := os.ReadFile("/opt/public_key.pem")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "get public key success", "data": strings.TrimSpace(string(data))})
}

func rsaDecode(input string) string {
	plainText, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		panic(err)
	}
	privateKey, _ := loadPrivateKey()
	decoded, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, plainText)
	if err != nil {
		panic(err)
	}
	return string(decoded)
}

func test3() {
	publicKey, _ := loadPublicKey()
	cipherText, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte("123"), nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(cipherText))
}
