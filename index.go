package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudflare/cloudflare-go"
	"github.com/pkg/errors"
)

type KVFile struct {
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

type KVFiles map[string]KVFile

type KVUploader struct {
	api *cloudflare.API
}

func (kvu *KVUploader) buildFilesMap(basePath string) (KVFiles, error) {
	var files = KVFiles{}

	_, err := os.Stat(basePath)
	if err != nil {
		return nil, errors.Wrap(err, "path not found")
	}

	err = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() == false {
			file, err := ioutil.ReadFile(path)

			if err != nil {
				return errors.Wrap(err, "error while reading file")
			}

			fileString := base64.StdEncoding.EncodeToString(file)
			pathWithoutBase := strings.Replace(path, basePath+"/", "", 1)

			files[pathWithoutBase] = KVFile{
				Content:     fileString,
				ContentType: http.DetectContentType(file),
			}
		}

		return nil
	})

	return files, err
}

func (kvu *KVUploader) uploadJSONToWorkersKV(namespaceID string, files KVFiles) error {
	for k, file := range files {
		fmt.Printf("Uploading file %v\n", k)

		payload, err := json.Marshal(file)
		if err != nil {
			return errors.Wrap(err, "error marshaling file definition")
		}

		_, err = kvu.api.WriteWorkersKV(context.Background(), namespaceID, k, payload)
		if err != nil {
			return errors.Wrap(err, "error while creating KV")
		}
	}

	return nil
}

func (kvu *KVUploader) findOrCreateNamespace(namespaceName string) (cloudflare.WorkersKVNamespace, error) {
	var namespace cloudflare.WorkersKVNamespace

	res, err := kvu.api.ListWorkersKVNamespaces(context.Background())
	if err != nil {
		return namespace, errors.Wrap(err, "error getting the list of namespace")
	}

	for _, value := range res.Result {
		if value.Title == namespaceName {
			namespace = value
		}
	}

	if namespace.ID == "" {
		fmt.Printf("Namespace not found, creating %v", namespaceName)
		req := &cloudflare.WorkersKVNamespaceRequest{Title: namespaceName}
		res, err := kvu.api.CreateWorkersKVNamespace(context.Background(), req)

		if err != nil {
			return namespace, errors.Wrap(err, "error with creating namespace")
		}

		namespace = res.Result
	}

	return namespace, nil
}

func validateEnvironmentVariables(variableKeys []string) error {
	var err string
	for _, key := range variableKeys {
		if _, ok := os.LookupEnv(key); !ok {
			err = err + fmt.Sprintf("%v not found\n", key)
		}
	}

	if len(err) > 0 {
		return errors.New(err)
	}

	return nil
}

func main() {
	requiredEnvVars := []string{
		"CF_API_KEY",
		"CF_API_EMAIL",
		"TARGET_DIRECTORY",
		"CF_API_ACCOUNT_ID",
		"CF_KV_NAMESPACE",
	}

	err := validateEnvironmentVariables(requiredEnvVars)
	if err != nil {
		log.Fatalf("missing required env vars %v", err)
	}

	cf, err := cloudflare.New(os.Getenv("CF_API_KEY"), os.Getenv("CF_API_EMAIL"), cloudflare.UsingAccount(os.Getenv("CF_API_ACCOUNT_ID")))
	if err != nil {
		log.Fatalf("error initializing cf client %v", err)
	}

	var kvu = KVUploader{
		api: cf,
	}

	files, err := kvu.buildFilesMap(os.Getenv("TARGET_DIRECTORY"))
	if err != nil {
		log.Fatalf("error walking the path: %v", err)
	}

	namespace, err := kvu.findOrCreateNamespace(os.Getenv("CF_KV_NAMESPACE"))
	if err != nil {
		log.Fatalf("error while finding or creating namespace %v", err)
	}

	err = kvu.uploadJSONToWorkersKV(namespace.ID, files)
	if err != nil {
		log.Fatalf("error while uploading to WorkersKV %v", err)
	}

	fmt.Print("All values written to WorkersKV successfully")
}
