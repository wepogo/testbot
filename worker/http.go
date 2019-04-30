package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	s3pkg "github.com/aws/aws-sdk-go/service/s3"
	"golang.org/x/xerrors"
)

var textPlainUTF8 = "text/plain; charset=utf-8"

type statusError int

func (e statusError) Error() string {
	return http.StatusText(int(e))
}

func postJSON(path string, in, out interface{}) error {
	reqBody := new(bytes.Buffer)
	json.NewEncoder(reqBody).Encode(in)
	resp, err := httpClient.Post(
		farmerURL+path,
		"application/json; charset=utf-8",
		reqBody,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "POST "+path+" errored", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		fmt.Fprintln(os.Stderr, "POST "+path+" failed", resp)
		return statusError(resp.StatusCode)
	}
	if out != nil {
		err = json.NewDecoder(resp.Body).Decode(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "decoding error ", err)
			return err
		}
	}
	return nil
}

func uploadToS3(f *os.File) (url string, err error) {
	key := "testbot/" + filepath.Base(f.Name()) + "." + boxID
	_, err = s3.PutObject(&s3pkg.PutObjectInput{
		ACL:         aws.String("public-read"),
		Bucket:      &bucket,
		Key:         &key,
		Body:        f,
		ContentType: &textPlainUTF8,
	})
	if err != nil {
		return "", xerrors.Errorf("bucket : %w", err)
	}

	return "https://" + bucket + ".s3.amazonaws.com/" + key, nil
}
