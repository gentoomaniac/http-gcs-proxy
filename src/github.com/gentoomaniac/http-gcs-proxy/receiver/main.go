package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"../libs/encryption"
	"../libs/env"
	"../libs/gcs"

	"golang.org/x/crypto/openpgp"

	"cloud.google.com/go/storage"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("main")
var recipient *openpgp.Entity
var ctx context.Context
var bucket *storage.BucketHandle
var bucketAttrs *storage.BucketAttrs

func encryptAndUploadAsync(fileBytes []byte, customMetadata map[string]string, gcsObjectName string) {
	log.Debugf("GPG encrypting file ...")
	encrypted, err := encryption.PgpEncrypt(fileBytes, []*openpgp.Entity{recipient}, nil)
	if err != nil {
		log.Errorf("%s", err)
		return
	}

	log.Debugf("Uploading object: %s", gcsObjectName)
	_, _, err = gcs.SendToGCS(ctx, bucket, gcsObjectName, bytes.NewBuffer(encrypted), customMetadata, false)
	if err != nil {
		log.Errorf("%s", err)
		return
	}
	log.Debugf("File successfully uploaded")
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

	gcsObjectName := r.Header.Get("Object-Name")
	if gcsObjectName == "" {
		gcsObjectName = fmt.Sprintf("%x", sha256.Sum256(fileBytes))
	}
	gcsObjectPath := r.Header.Get("Object-Path")
	if gcsObjectPath != "" {
		gcsObjectName = strings.TrimPrefix(gcsObjectPath, "/") + "/" + gcsObjectName
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

	response := map[string]interface{}{}
	asyncUpload := r.Header.Get("Async-processing")
	if asyncUpload == "true" {
		go encryptAndUploadAsync(fileBytes, customMetadata, gcsObjectName)
		response = map[string]interface{}{
			"status":     "started",
			"details":    "upload in progress",
			"objectName": gcsObjectName,
			"uri":        fmt.Sprintf("gcs://%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
			"linkUrl":    fmt.Sprintf("https://storage.cloud.google.com/%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
		}
		w.WriteHeader(http.StatusOK)
	} else {
		encryptAndUploadAsync(fileBytes, customMetadata, gcsObjectName)
		response = map[string]interface{}{
			"status":     "success",
			"details":    "file successfully uploaded",
			"objectName": gcsObjectName,
			"uri":        fmt.Sprintf("gcs://%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
			"linkUrl":    fmt.Sprintf("https://storage.cloud.google.com/%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
		}
		w.WriteHeader(http.StatusOK)
	}

	js, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Debugf("Request processed")
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func main() {
	loggo.ConfigureLoggers(env.GetenvDefault("LOGGING_CONFIG", "main=DEBUG"))

	rec, err := encryption.ReadEntity(env.GetenvDefault("PGP_PUBLIC_KEY", "/tmp/pubKey.asc"))
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

	bucketName := env.GetenvDefault("GCS_BUCKET", "encrypted_data")
	bucket = client.Bucket(bucketName)
	if bucketAttrs, err = bucket.Attrs(ctx); err != nil {
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
	http.ListenAndServe(fmt.Sprintf("%s:%s", env.GetenvDefault("LISTEN_ADDRESS", "127.0.0.1"), env.GetenvDefault("LISTEN_PORT", "8080")), nil)
}
