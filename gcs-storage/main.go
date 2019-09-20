// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"

	"cloud.google.com/go/storage"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("main")
var recipient *openpgp.Entity
var ctx context.Context
var bucket *storage.BucketHandle

type Response struct {
	status  string
	payload string
}

func sendToGCS(ctx context.Context, bucketObject *storage.BucketHandle, objectName string, r io.Reader, metadata map[string]string, public bool) (*storage.ObjectHandle, *storage.ObjectAttrs, error) {
	log.Debugf("Sending encrypted data to GCS: %s", objectName)
	obj := bucketObject.Object(objectName)

	w := obj.NewWriter(ctx)
	newAttrs := w.ObjectAttrs
	newAttrs.Metadata = metadata
	w.ObjectAttrs = newAttrs

	if _, err := io.Copy(w, r); err != nil {
		log.Errorf("%s", err)
		return nil, nil, err
	}

	if err := w.Close(); err != nil {
		log.Errorf("%s", err)
		return nil, nil, err
	}

	if public {
		if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
			log.Errorf("%s", err)
			return nil, nil, err
		}
	}

	attrs, err := obj.Attrs(ctx)
	return obj, attrs, err
}

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

func pgpEncrypt(data []byte, recip []*openpgp.Entity, signer *openpgp.Entity) ([]byte, error) {
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

func uploadFile(w http.ResponseWriter, r *http.Request) {
	log.Debugf("File Upload Endpoint Hit")

	// Parse our multipart form, 10 << 20 specifies a maximum
	// upload of 10 MB files.
	r.ParseMultipartForm(10 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Errorf("Error Retrieving the File")
		log.Errorf("%s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Errorf("%s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	checksum := sha256.Sum256(fileBytes)

	encrypted, err := pgpEncrypt(fileBytes, []*openpgp.Entity{recipient}, nil)
	if err != nil {
		log.Errorf("%s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	customMetadata := make(map[string]string)
	for k, values := range r.Header {
		if strings.HasPrefix(k, "__") {
			value := ""
			if len(values) > 0 {
				value = values[0]
			}
			log.Debugf("%s: %v", k, value)
			customMetadata[k] = value
		}
	}

	gcsObjectName := r.Header.Get("Object-Name")
	if gcsObjectName == "" {
		gcsObjectName = fmt.Sprintf("%x", sha256.Sum256(fileBytes))
	}
	gcsObjectPath := r.Header.Get("Object-Path")
	if gcsObjectPath != "" {
		gcsObjectName = strings.TrimPrefix(gcsObjectPath, "/") + "/" + gcsObjectName
	}
	log.Debugf("Uploading object to %s", gcsObjectName)

	_, _, err = sendToGCS(ctx, bucket, gcsObjectName, bytes.NewBuffer(encrypted), customMetadata, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := Response{
		status:  "success",
		payload: fmt.Sprintf("%x", checksum),
	}

	js, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func getenv(name string, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {
	loggo.ConfigureLoggers(getenv("LOGGING_CONFIG", "main=DEBUG"))

	rec, err := readEntity(getenv("PGP_PUBLIC_KEY", "/tmp/pubKey.asc"))
	if err != nil {
		fmt.Println(err)
		return
	}
	recipient = rec

	ctx = context.Background()
	//	_, objAttrs, err := upload(ctx, r, projectID, bucket, name, public)

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Errorf("Could not create GCS client: %s", err)
		os.Exit(1)
	}

	bucketName := getenv("GCS_BUCKET", "encrypted_data")
	bucket = client.Bucket(bucketName)
	if _, err = bucket.Attrs(ctx); err != nil {
		switch err {
		case storage.ErrBucketNotExist:
			log.Errorf("Bucket doesn't exist: %s", bucketName)
			os.Exit(2)
		default:
			log.Errorf("Unknown error: %s", err)
			os.Exit(42)
		}
	}

	log.Debugf("starting server ...")

	//fs := http.FileServer(http.Dir("static/"))
	//http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/upload", uploadFile)
	http.ListenAndServe(fmt.Sprintf("%s:%s", getenv("LISTEN_ADDRESS", "127.0.0.1"), getenv("LISTEN_PORT", "8080")), nil)
}
