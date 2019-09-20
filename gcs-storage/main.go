// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"

	"github.com/juju/loggo"
)

var log = loggo.GetLogger("main")
var recipient *openpgp.Entity

func readEntity(name string) (*openpgp.Entity, error) {
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

func pgpEncrypt(data []byte, recip []*openpgp.Entity, signer *openpgp.Entity) (string, error) {
	log.Debugf("Encrypting %d bytes ...", len(data))

	encbuf := bytes.NewBuffer(nil)
	armorWriter, err := armor.Encode(encbuf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", err
	}

	pgpMessage, err := openpgp.Encrypt(armorWriter, recip, nil, nil, nil)
	if err != nil {
		return "", err
	}
	encryptedBytes, err := pgpMessage.Write(data)
	if err != nil {
		return "", err
	}

	pgpMessage.Close()
	armorWriter.Close()

	log.Debugf("Encrypted bytes: %d", encryptedBytes)

	return encbuf.String(), err
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	log.Debugf("File Upload Endpoint Hit")
	cachePath := getenv("LOCAL_CACHE", "/tmp")

	// Parse our multipart form, 10 << 20 specifies a maximum
	// upload of 10 MB files.
	r.ParseMultipartForm(10 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Errorf("Error Retrieving the File")
		log.Errorf("%s", err)
		return
	}
	defer file.Close()
	log.Infof("Uploaded File: %+v\n", header.Filename)
	log.Infof("File Size: %+v\n", header.Size)
	log.Infof("MIME Header: %+v\n", header.Header)

	// read all of the contents of our uploaded file into a
	// byte array
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Errorf("%s", err)
	}
	checksum := sha256.Sum256(fileBytes)

	tempFile, err := os.Create(fmt.Sprintf("%v/%x", cachePath, checksum))
	if err != nil {
		log.Errorf("%s", err)
	}
	defer tempFile.Close()

	encrypted, err := pgpEncrypt(fileBytes, []*openpgp.Entity{recipient}, nil)
	if err != nil {
		log.Errorf("%s", err)
	}

	log.Debugf("Writing encrypted data to file %d ...", len(encrypted))
	tempFile.WriteString(encrypted)
	tempFile.Close()

	fmt.Printf("{\"status\": \"success\", \"filename\":\"%x\"}", checksum)
}

func getenv(name string, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {
	loggo.ConfigureLoggers(getenv("LOGGING_CONFIG", "*=TRACE"))

	rec, err := readEntity(getenv("PGP_PUBLIC_KEY", "/tmp/pubKey.asc"))
	if err != nil {
		fmt.Println(err)
		return
	}
	recipient = rec

	http.HandleFunc("/upload", uploadFile)

	//fs := http.FileServer(http.Dir("static/"))
	//http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Debugf("starting server ...")
	http.ListenAndServe(":8080", nil)
}
