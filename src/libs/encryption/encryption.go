package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"

	"github.com/juju/loggo"
)

var log = loggo.GetLogger("encryption")

func AES256(data []byte, key []byte) (ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	//nonce := make([]byte, 12)
	//if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
	//	return
	//}
	nonce, _ := hex.DecodeString("64a9433eae7ccceee2fc0eda") // ToDo: this needs to get exchanged

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
