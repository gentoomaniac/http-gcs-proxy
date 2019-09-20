package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("main")

// Creates a new file upload http request with optional extra params
func newfileUploadRequest(uri string, path string, headers []string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, header := range headers {
		fields := strings.SplitN(header, ":", 2)
		k := fields[0]
		v := ""
		if len(fields) > 1 {
			v = fields[1]
		}

		req.Header.Set(k, v)
		log.Debugf("setting header: '%s: %s'", k, v)
	}

	return req, err
}

func getenv(name string, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

var (
	verbose = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	file    = kingpin.Flag("file", "File to be uploaded").Short('f').Required().ExistingFile()
	headers = kingpin.Flag("header", "Set a header").Short('h').Strings()
	url     = kingpin.Flag("url", "URL to send the request to").Short('u').Required().String()
)

func main() {
	loggo.ConfigureLoggers(getenv("LOGGING_CONFIG", "main=DEBUG"))
	kingpin.Version("0.0.1")
	kingpin.Parse()

	log.Debugf("%s", headers)
	request, err := newfileUploadRequest(*url, *file, *headers)
	if err != nil {
		log.Errorf("%s", err)
	}
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		log.Errorf("%s", err)
	} else {
		body := &bytes.Buffer{}
		_, err := body.ReadFrom(resp.Body)
		if err != nil {
			log.Errorf("%s", err)
		}
		resp.Body.Close()
		log.Debugf("Status: %d", resp.StatusCode)
		for k, v := range resp.Header {
			log.Debugf("%s: %s", k, v)
		}
		log.Infof("Body: %s", body)
	}
}
