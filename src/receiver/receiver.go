package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"../libs/encryption"
	"../libs/env"
	"../libs/gcs"

	"golang.org/x/crypto/openpgp"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("main")
var recipient *openpgp.Entity
var ctx context.Context
var bucket *storage.BucketHandle
var bucketAttrs *storage.BucketAttrs
var keyEncryptionKey []byte

func sendResponse(response map[string]interface{}, httpStatus int, w http.ResponseWriter) {
	js, err := json.Marshal(response)
	if err != nil {
		log.Errorf("Error processing json response: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(httpStatus)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func encryptAndUploadAsync(fileBytes []byte, customMetadata map[string]string, gcsObjectName string, encryptionType string, passphrase string) (err error) {
	var data []byte
	var encryptedKey []byte
	switch encryptionType {

	case "pubkey":
		log.Debugf("Encrypting data with gpg pubkey ...")
		data, err = encryption.PgpPubkey(fileBytes, []*openpgp.Entity{recipient}, nil)
		if err != nil {
			log.Errorf("Encryption failed: %s", err)
			return
		}
	case "symmetric":
		log.Debugf("Encrypting data with gpg symmetric key ...")
		data, err = encryption.PgpSymmetric(fileBytes, passphrase)
		if err != nil {
			log.Errorf("Encryption failed: %s", err)
			return
		}
	case "plain":
		log.Debugf("AES256 encrypting data with symmetric key ...")
		var fileKey []byte
		fileKey, err = encryption.GenerateAES256Key()
		if err != nil {
			log.Errorf("Didn't get key for the file: %s", err)
			return
		}
		data, err = encryption.AES256(fileBytes, fileKey)
		if err != nil {
			log.Errorf("Encryption failed: %s", err)
			return
		}
		encryptedKey, err = encryption.AES256(fileKey, keyEncryptionKey)
		if err != nil {
			log.Errorf("Couldn't encrypt file key: %s", err)
			return
		}
		log.Debugf("encrypted key: %s", base64.StdEncoding.EncodeToString(encryptedKey))
		customMetadata["_encryptedKey"] = base64.StdEncoding.EncodeToString(encryptedKey)
	default:
		log.Debugf("Skipping encryption ...")
	}

	log.Debugf("Uploading object: %s", gcsObjectName)
	_, _, err = gcs.SendToGCS(ctx, bucket, gcsObjectName, bytes.NewBuffer(data), customMetadata, false)
	if err != nil {
		log.Errorf("%s", err)
		return
	}
	log.Debugf("File successfully uploaded")
	return
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	log.Debugf("File Upload Endpoint Hit")

	vars := mux.Vars(r)
	log.Debugf("Path: %s", vars["path"])
	gcsObjectName := vars["path"]

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
	encryptionType := strings.ToLower(r.Header.Get("Encryption-Type"))
	symmkey := ""
	switch encryptionType {
	case "none":
	case "plain":
	case "pubkey":
	case "symmetric":
		symmkey = r.Header.Get("Symmkey")
		if symmkey == "" {
			log.Errorf("No value for 'Symmkey'")
			response = map[string]interface{}{
				"status":  "missing_parameters",
				"details": "'Symmkey' not specified",
			}
			sendResponse(response, http.StatusBadRequest, w)
			return
		}
	case "":
		log.Errorf("No value for 'Encryption-Type'")
		response = map[string]interface{}{
			"status":  "missing_parameters",
			"details": "'Encryption-Type' not specified",
		}
		sendResponse(response, http.StatusBadRequest, w)
		return
	default:
		log.Errorf("Value for 'Encryption-Type' invalid value: %s", encryptionType)
		response = map[string]interface{}{
			"status":  "not_supported",
			"details": "Value for 'Encryption-Type' not supported",
		}
		sendResponse(response, http.StatusBadRequest, w)
		return
	}
	asyncUpload := r.Header.Get("Async-processing")
	if asyncUpload == "true" {
		go encryptAndUploadAsync(fileBytes, customMetadata, gcsObjectName, encryptionType, symmkey)
		response = map[string]interface{}{
			"status":     "started",
			"details":    "upload in progress",
			"objectName": gcsObjectName,
			"uri":        fmt.Sprintf("gcs://%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
			"linkUrl":    fmt.Sprintf("https://storage.cloud.google.com/%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
		}
		sendResponse(response, http.StatusOK, w)
	} else {
		encryptAndUploadAsync(fileBytes, customMetadata, gcsObjectName, encryptionType, symmkey)
		response = map[string]interface{}{
			"status":     "success",
			"details":    "file successfully uploaded",
			"objectName": gcsObjectName,
			"uri":        fmt.Sprintf("gcs://%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
			"linkUrl":    fmt.Sprintf("https://storage.cloud.google.com/%s/%s", env.GetenvDefault("GCS_BUCKET", "encrypted_data"), gcsObjectName),
		}
		sendResponse(response, http.StatusOK, w)
	}
	return
}

func getFile(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Get File Endpoint Hit")

	vars := mux.Vars(r)
	log.Debugf("Path: %s", vars["path"])

	data := gcs.ReadFile(ctx, bucket, vars["path"])

	if data != nil {
		log.Debugf("Request processed")
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Disposition", "attachment; filename="+path.Base(vars["path"]))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}

}

func main() {
	loggo.ConfigureLoggers(env.GetenvDefault("LOGGING_CONFIG", "main=DEBUG"))

	rec, err := encryption.ReadEntity(env.GetenvDefault("PGP_PUBLIC_KEY", "/tmp/pubKey.asc"))
	if err != nil {
		log.Errorf("Could not read public key: %s", err)
		os.Exit(3)
	}
	recipient = rec

	keyEncryptionKey, err = encryption.GenerateAES256Key()
	if err != nil {
		log.Errorf("Could not generate key: %s", err)
		os.Exit(4)
	}
	log.Infof("Generated key: %s", base64.StdEncoding.EncodeToString(keyEncryptionKey))

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
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/{path:.*}", uploadFile).Methods("POST")
	router.HandleFunc("/{path:.*}", getFile).Methods("GET")
	http.ListenAndServe(fmt.Sprintf("%s:%s", env.GetenvDefault("LISTEN_ADDRESS", "127.0.0.1"), env.GetenvDefault("LISTEN_PORT", "8080")), router)
}
