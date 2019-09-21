package encryption

import (
	"bytes"
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
