package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"

	"github.com/juju/loggo"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

var log = loggo.GetLogger("encryption")

func ReadEntity(name string) (*openpgp.Entity, error) {
	log.Debugf("loading entity: %s", name)
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	block, err := armor.Decode(f)
	if err != nil {
		return nil, err
	}
	return openpgp.ReadEntity(packet.NewReader(block.Body))
}

func PgpEncrypt(data []byte, recip []*openpgp.Entity, signer *openpgp.Entity) ([]byte, error) {
	log.Debugf("Encrypting %d bytes ...", len(data))

	encbuf := bytes.NewBuffer(nil)
	pgpMessage, err := openpgp.Encrypt(encbuf, recip, nil, &openpgp.FileHints{IsBinary: true}, nil)
	if err != nil {
		return nil, err
	}
	_, err = pgpMessage.Write(data)
	if err != nil {
		return nil, err
	}
	pgpMessage.Close()

	return encbuf.Bytes(), err
}

func Encrypt(plaintext []byte, password []byte, packetConfig *packet.Config) (ciphertext []byte, err error) {

	encbuf := bytes.NewBuffer(nil)

	pt, _ := openpgp.SymmetricallyEncrypt(encbuf, password, nil, packetConfig)

	_, err = pt.Write(plaintext)
	if err != nil {
		return
	}

	pt.Close()
	ciphertext = encbuf.Bytes()

	return
}

func AES256(data []byte, secret string) []byte {
	key, _ := hex.DecodeString(secret)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	ciphertext := aesgcm.Seal(nil, nonce, data, nil)
	return ciphertext
}
