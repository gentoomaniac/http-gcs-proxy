package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

	"github.com/juju/loggo"
)

var log = loggo.GetLogger("encryption")

func AES256(data []byte, key []byte, nonce []byte) (ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	ciphertext = aesgcm.Seal(nil, nonce, data, nil)
	return
}

func GenerateAES256Key() (key []byte, err error) {
	key = make([]byte, 32)

	_, err = rand.Read(key)
	if err != nil {
		log.Errorf("Failed to generate key: %s", err)
	}
	return
}

func GenerateIV() (iv []byte, err error) {
	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	//nonce := make([]byte, 12)
	//if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
	//	return
	//}
	iv = make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err.Error())
	}
	return
}
