package object

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)


func TestR2Storage(t *testing.T) {

	endpoint := ""
	bucket := ""
	accessKey := ""
	secretKey := ""


	r2, err := newR2(bucket, accessKey, secretKey, endpoint)
	if err != nil {
		t.Fatalf("❌ Failed to initialize R2 storage: %v", err)
	}
	fmt.Println("✅ Successfully initialized R2 storage!")


	key := "test_file.txt"
	data := "Hello, JuiceFS R2!"


	err = r2.Put(key, bytes.NewReader([]byte(data)))
	if err != nil {
		t.Fatalf("❌ Failed to upload object to R2: %v", err)
	}
	fmt.Println("✅ Successfully uploaded test file to R2!")


	reader, err := r2.Get(key, 0, int64(len(data)))
	if err != nil {
		t.Fatalf("❌ Failed to retrieve object from R2: %v", err)
	}
	defer reader.Close()


	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("❌ Failed to read object content: %v", err)
	}

	if string(content) != data {
		t.Fatalf("❌ Data mismatch: expected %s, got %s", data, string(content))
	}
	fmt.Println("✅ Successfully retrieved and verified test file!")


	err = r2.Delete(key)
	if err != nil {
		t.Fatalf("❌ Failed to delete object from R2: %v", err)
	}
	fmt.Println("✅ Successfully deleted test file from R2!")
}
