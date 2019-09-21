package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"

	"../libs/env"
	"../libs/gcs"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("main")
var ctx context.Context
var bucket *storage.BucketHandle
var bucketAttrs *storage.BucketAttrs

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

	ctx = context.Background()

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
	router.HandleFunc("/{path:.*}", getFile).Methods("GET")
	http.ListenAndServe(fmt.Sprintf("%s:%s", env.GetenvDefault("LISTEN_ADDRESS", "127.0.0.1"), env.GetenvDefault("LISTEN_PORT", "8080")), router)

}
