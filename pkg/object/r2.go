package object

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type r2Client struct {
	bucket  string
	s3      *s3.S3
	session *session.Session
}

func newR2(endpoint, accessKey, secretKey, bucket string) (ObjectStorage, error) {
	if !strings.Contains(endpoint, "r2.cloudflarestorage.com") {
		return nil, fmt.Errorf("invalid R2 endpoint: %s", endpoint)
	}

	awsConfig := &aws.Config{
		Endpoint:         aws.String(endpoint),
		Region:           aws.String("auto"),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create R2 session: %v", err)
	}

	return &r2Client{
		bucket:  bucket,
		s3:      s3.New(sess),
		session: sess,
	}, nil
}

func (r *r2Client) String() string {
	return fmt.Sprintf("r2://%s/", r.bucket)
}

func (r *r2Client) Limits() Limits {
	return Limits{
		IsSupportMultipartUpload: true,
		IsSupportUploadPartCopy:  false,
		MinPartSize:              5 << 20,
		MaxPartSize:              5 << 30,
		MaxPartCount:             10000,
	}
}

func (r *r2Client) Head(key string) (Object, error) {
	params := &s3.HeadObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	}
	resp, err := r.s3.HeadObject(params)
	if err != nil {
		return nil, err
	}

	return &obj{
		key:   key,
		size:  *resp.ContentLength,
		mtime: *resp.LastModified,
		isDir: strings.HasSuffix(key, "/"),
	}, nil
}

func (r *r2Client) Get(key string, off, limit int64, getters ...AttrGetter) (io.ReadCloser, error) {
	params := &s3.GetObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	}

	
	if off > 0 || limit > 0 {
		var rangeHeader string
		if limit > 0 {
			rangeHeader = fmt.Sprintf("bytes=%d-%d", off, off+limit-1)
		} else {
			rangeHeader = fmt.Sprintf("bytes=%d-", off)
		}
		params.Range = &rangeHeader
	}

	resp, err := r.s3.GetObject(params)
	if err != nil {
		return nil, err
	}

	attrs := applyGetters(getters...)
	attrs.SetRequestID("R2_Get") 

	return resp.Body, nil
}


func (r *r2Client) Put(key string, in io.Reader, getters ...AttrGetter) error {
	var body io.ReadSeeker
	if b, ok := in.(io.ReadSeeker); ok {
		body = b
	} else {
		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	params := &s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
		Body:   body,
	}

	_, err := r.s3.PutObject(params)

	
	attrs := applyGetters(getters...)
	attrs.SetRequestID("R2_Put") 

	return err
}


func (r *r2Client) Delete(key string, getters ...AttrGetter) error {
	params := &s3.DeleteObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	}

	_, err := r.s3.DeleteObject(params)


	attrs := applyGetters(getters...)
	attrs.SetRequestID("R2_Delete") 

	return err
}

func (r *r2Client) List(prefix, start, token, delimiter string, limit int64, followLink bool) ([]Object, bool, string, error) {
	params := &s3.ListObjectsV2Input{
		Bucket:       aws.String(r.bucket),
		Prefix:       aws.String(prefix),
		MaxKeys:      aws.Int64(limit),
		EncodingType: aws.String("url"),
	}

	if start != "" {
		params.StartAfter = aws.String(start)
	}
	if token != "" {
		params.ContinuationToken = aws.String(token)
	}
	if delimiter != "" {
		params.Delimiter = aws.String(delimiter)
	}

	resp, err := r.s3.ListObjectsV2(params)
	if err != nil {
		return nil, false, "", err
	}

	var objs []Object
	for _, o := range resp.Contents {
		key, _ := url.QueryUnescape(*o.Key)
		objs = append(objs, &obj{
			key:   key,
			size:  *o.Size,
			mtime: *o.LastModified,
			isDir: strings.HasSuffix(key, "/"),
		})
	}

	sort.Slice(objs, func(i, j int) bool { return objs[i].Key() < objs[j].Key() })

	nextToken := ""
	if resp.NextContinuationToken != nil {
		nextToken = *resp.NextContinuationToken
	}

	return objs, *resp.IsTruncated, nextToken, nil
}

func (r *r2Client) Copy(dst, src string) error {
	srcPath := r.bucket + "/" + src
	params := &s3.CopyObjectInput{
		Bucket:     aws.String(r.bucket),
		Key:        aws.String(dst),
		CopySource: aws.String(srcPath),
	}

	_, err := r.s3.CopyObject(params)
	return err
}

func (r *r2Client) Create() error {
	return fmt.Errorf("Cloudflare R2 does not support creating buckets via API")
}

func (r *r2Client) CompleteUpload(key string, uploadID string, parts []*Part) error {
	return nil
}

func (r *r2Client) AbortUpload(key string, uploadID string) {
	// not support
}

func (r *r2Client) CreateMultipartUpload(key string) (*MultipartUpload, error) {
	return nil, fmt.Errorf("Cloudflare R2 does not support multipart upload")
}

func (r *r2Client) ListAll(prefix, marker string, followLink bool) (<-chan Object, error) {
	return nil, fmt.Errorf("Cloudflare R2 does not support ListAll")
}
func (r *r2Client) ListUploads(marker string) ([]*PendingPart, string, error) {
	return nil, "", fmt.Errorf("Cloudflare R2 does not support ListUploads")
}
func (r *r2Client) UploadPart(key string, uploadID string, num int, body []byte) (*Part, error) {
	return nil, fmt.Errorf("Cloudflare R2 does not support multipart upload")
}
func (r *r2Client) UploadPartCopy(key string, uploadID string, num int, srcKey string, off, size int64) (*Part, error) {
	return nil, fmt.Errorf("Cloudflare R2 does not support multipart upload or part copy")
}

func init() {
	Register("r2", newR2)
}