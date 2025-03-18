package object

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/juicedata/juicefs/pkg/utils"
)

type r2client struct {
	bucket  string
	s3      *s3.S3
	ses     *session.Session
}

func (r *r2client) String() string {
	return fmt.Sprintf("r2://%s/", r.bucket)
}

func (r *r2client) Limits() Limits {
	return Limits{
		IsSupportMultipartUpload: true,
		IsSupportUploadPartCopy:  false,
		MinPartSize:              5 << 20,
		MaxPartSize:              5 << 30,
		MaxPartCount:             10000,
	}
}

func newR2(endpoint, accessKey, secretKey, bucket string) (ObjectStorage, error) {
	if !strings.Contains(endpoint, "r2.cloudflarestorage.com") {
		return nil, fmt.Errorf("invalid R2 endpoint: %s", endpoint)
	}

	awsConfig := &aws.Config{
		Endpoint:         aws.String(endpoint),
		Region:           aws.String("auto"), // R2 没有 region 概念
		S3ForcePathStyle: aws.Bool(true),     // R2 需要 Path-Style 访问
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create R2 session: %v", err)
	}

	return &r2client{
		bucket:  bucket,
		s3:      s3.New(sess),
		ses:     sess,
	}, nil
}

func (r *r2client) Head(key string) (Object, error) {
	param := s3.HeadObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	}
	resp, err := r.s3.HeadObject(&param)
	if err != nil {
		return nil, err
	}

	return &obj{
		key,
		*resp.ContentLength,
		*resp.LastModified,
		strings.HasSuffix(key, "/"),
		"",
	}, nil
}

func (r *r2client) Get(key string, off, limit int64) (io.ReadCloser, error) {
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

	return resp.Body, nil
}

func (r *r2client) Put(key string, in io.Reader) error {
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
		Bucket: &r.bucket,
		Key:    &key,
		Body:   body,
	}

	_, err := r.s3.PutObject(params)
	return err
}

func (r *r2client) Delete(key string) error {
	param := s3.DeleteObjectInput{
		Bucket: &r.bucket,
		Key:    &key,
	}

	_, err := r.s3.DeleteObject(&param)
	return err
}

func (r *r2client) List(prefix, start, token, delimiter string, limit int64, followLink bool) ([]Object, bool, string, error) {
	param := s3.ListObjectsV2Input{
		Bucket:       &r.bucket,
		Prefix:       &prefix,
		MaxKeys:      aws.Int64(limit),
		EncodingType: aws.String("url"),
	}

	if start != "" {
		param.StartAfter = aws.String(start)
	}
	if token != "" {
		param.ContinuationToken = aws.String(token)
	}
	if delimiter != "" {
		param.Delimiter = aws.String(delimiter)
	}

	resp, err := r.s3.ListObjectsV2(&param)
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

func (r *r2client) Copy(dst, src string) error {
	srcPath := r.bucket + "/" + src
	params := &s3.CopyObjectInput{
		Bucket:     aws.String(r.bucket),
		Key:        aws.String(dst),
		CopySource: aws.String(srcPath),
	}

	_, err := r.s3.CopyObject(params)
	return err
}

func (r *r2client) Create() error {
	return fmt.Errorf("Cloudflare R2 does not support creating buckets via API")
}

func init() {
	Register("r2", newR2)
}