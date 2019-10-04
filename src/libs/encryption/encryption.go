package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
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

func PgpPubkey(data []byte, recip []*openpgp.Entity, signer *openpgp.Entity) ([]byte, error) {
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

func PgpSymmetric(data []byte, password string) (ciphertext []byte, err error) {
	encbuf := bytes.NewBuffer(nil)
	packetConfig := &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}
	pt, _ := openpgp.SymmetricallyEncrypt(encbuf, []byte(password), &openpgp.FileHints{IsBinary: true}, packetConfig)

	_, err = pt.Write(data)
	if err != nil {
		return
	}

	pt.Close()
	ciphertext = encbuf.Bytes()
	return
}

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
