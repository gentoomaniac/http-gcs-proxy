package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"io/ioutil"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("main")

func getenv(name string, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

var (
	verbose = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	secret  = kingpin.Flag("secret", "Secret").Short('s').Required().String()
	nonce   = kingpin.Flag("nonce", "Secret").Short('n').Required().String()
)

func decrypt(ciphertext []byte, key []byte, nonce []byte) (decryptedData []byte) {

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	decryptedData, err = aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}

	return
}

func main() {
	loggo.ConfigureLoggers(getenv("LOGGING_CONFIG", "main=DEBUG"))
	kingpin.Version("0.0.1")
	kingpin.Parse()

	encryptedData, _ := ioutil.ReadAll(os.Stdin)
	key, _ := base64.StdEncoding.DecodeString(*secret)
	iv, _ := base64.StdEncoding.DecodeString(*nonce)
	os.Stdout.Write(decrypt(encryptedData, key, iv))
}
