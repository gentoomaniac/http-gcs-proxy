package gcs

import (
	"context"
	"io"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("gcs")

func SendToGCS(ctx context.Context, bucketObject *storage.BucketHandle, objectName string, r io.Reader, metadata map[string]string, public bool) (*storage.ObjectHandle, *storage.ObjectAttrs, error) {
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

// readFile reads the named file in Google Cloud Storage.
func ReadFile(ctx context.Context, bucketObject *storage.BucketHandle, fileName string) []byte {
	rc, err := bucketObject.Object(fileName).NewReader(ctx)
	if err != nil {
		log.Errorf("unable to open file from bucket: %s, %v", fileName, err)
		return nil
	}
	defer rc.Close()
	data, err := ioutil.ReadAll(rc)
	if err != nil {
		log.Errorf("unable to read data from bucket: %s, %v", fileName, err)
		return nil
	}

	return data
}
