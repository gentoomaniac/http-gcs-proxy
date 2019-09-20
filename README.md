# HTTP GCS Proxy

## Disclaimer

This project is built mainly for learning purposes.

If you want to use if for production purposes, this is at your own risk.

## What

This repo contains two components.

- a server that is receiving files via http and uploads them gpg encrypted to a GCS bucket
- a script to upload files via http POST to a webserver (mainly for testing the server)

## Why

As mentioned in the disclaimer, this is mainly for learning purposes.

However the idea is based on a real world problem in a professional environment.

## How

### Server

The main component, receiving and writing the data to GCP

#### Configuration

```shell
# configure the logging (github.com/juju/loggo)
export LOGGING_CONFIG=main=DEBUG

# Path to a GPG publick key with which the data will get encrypted
export PGP_PUBLIC_KEY=/home/user/pubKey.asc

# GCP service account credentials as described in the GCP documentation
export GOOGLE_APPLICATION_CREDENTIALS="/home/user/encryption_service_platform-226210-405397855379.json"
```

#### Run

```shell
$ go run gcs-storage/main.go2019-09-20 10:29:22 DEBUG main main.go:68 loading entity: /home/user/pubKey.asc
2019-09-20 10:29:24 DEBUG main main.go:208 starting server ...
2019-09-20 10:29:25 DEBUG main main.go:99 File Upload Endpoint Hit
2019-09-20 10:29:25 DEBUG main main.go:82 Encrypting 9988477 bytes ...
2019-09-20 10:29:25 DEBUG main main.go:145 Uploading object to some/structure/146280ea3dc030eb6a0732778a9adb642065313a4080e56e13f9e6a6122de9d1
2019-09-20 10:29:25 DEBUG main main.go:38 Sending encrypted data to GCS: some/structure/146280ea3dc030eb6a0732778a9adb642065313a4080e56e13f9e6a6122de9d1

```

### Client

Simple client to upload the files. It allows for HTTP headers to be set.

#### Run

```shell
$ go run upload.go -u http://localhost:8080/upload -f /path/to/some.file -h foo:bar -h __fizz:buzz -h Object-path:/some/structure -h Object-name:foobar-fizz-buzz
2019-09-20 10:34:09 INFO main upload.go:95 Body: {}
```

Explanation:

- upload some.file
- Header `foo` with value `bar` will be sent but ignored by the server
- Heeader `__fizz` with value `buzz` will be sent and the server will set object metadata on the file with the key `__fizz` and the value `buzz` (default: bucket root directory)
- the object will be placed in the path `some/structure`
- the objects name will be `foobar-fizz-buzz` (default: sha256 hash of the file contents)
