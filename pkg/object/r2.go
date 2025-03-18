//go:build !nor2
// +build !nor2

/*
 * JuiceFS, Copyright 2018 Juicedata, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

 package object

 import (
	 "bytes"
	 "fmt"
	 "io"
	 "strings"
	 "time"
 
	 "github.com/aws/aws-sdk-go/aws"
	 "github.com/aws/aws-sdk-go/aws/credentials"
	 "github.com/aws/aws-sdk-go/aws/session"
	 "github.com/aws/aws-sdk-go/service/s3"
	 "github.com/juicedata/juicefs/pkg/utils"
 )
 
 // r2Client 结构体封装 Cloudflare R2
 type r2Client struct {
	 bucket  string
	 s3      *s3.S3
	 session *session.Session
 }
 
 // newR2 创建 Cloudflare R2 客户端
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
 
	 return &r2Client{
		 bucket:  bucket,
		 s3:      s3.New(sess),
		 session: sess,
	 }, nil
 }
 
 // 返回 R2 存储的 URL
 func (r *r2Client) String() string {
	 return fmt.Sprintf("r2://%s/", r.bucket)
 }
 
 // 上传文件到 R2
 func (r *r2Client) Put(key string, in io.Reader) error {
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
	 return err
 }
 
 // 获取文件
 func (r *r2Client) Get(key string, off, limit int64) (io.ReadCloser, error) {
	 params := &s3.GetObjectInput{
		 Bucket: aws.String(r.bucket),
		 Key:    aws.String(key),
	 }
 
	 // 处理 Range 读取
	 if off > 0 || limit > 0 {
		 var r string
		 if limit > 0 {
			 r = fmt.Sprintf("bytes=%d-%d", off, off+limit-1)
		 } else {
			 r = fmt.Sprintf("bytes=%d-", off)
		 }
		 params.Range = &r
	 }
 
	 resp, err := r.s3.GetObject(params)
	 if err != nil {
		 return nil, err
	 }
	 return resp.Body, nil
 }
 
 // 获取文件元数据
 func (r *r2Client) Head(key string) (Object, error) {
	 params := &s3.HeadObjectInput{
		 Bucket: aws.String(r.bucket),
		 Key:    aws.String(key),
	 }
 
	 resp, err := r.s3.HeadObject(params)
	 if err != nil {
		 return nil, err
	 }
 
	 return &obj{
		 key:   key,
		 size:  *resp.ContentLength,
		 mtime: *resp.LastModified,
		 isDir: false,
	 }, nil
 }
 
 // 删除文件
 func (r *r2Client) Delete(key string) error {
	 params := &s3.DeleteObjectInput{
		 Bucket: aws.String(r.bucket),
		 Key:    aws.String(key),
	 }
	 _, err := r.s3.DeleteObject(params)
	 return err
 }
 
 // 列出对象
 func (r *r2Client) List(prefix string, start string, token string, delimiter string, limit int64, followLink bool) ([]Object, bool, string, error) {
	 params := &s3.ListObjectsV2Input{
		 Bucket:       aws.String(r.bucket),
		 Prefix:       aws.String(prefix),
		 MaxKeys:      aws.Int64(limit),
		 ContinuationToken: aws.String(token),
		 Delimiter:    aws.String(delimiter),
	 }
 
	 resp, err := r.s3.ListObjectsV2(params)
	 if err != nil {
		 return nil, false, "", err
	 }
 
	 var objs []Object
	 for _, obj := range resp.Contents {
		 objs = append(objs, &obj{
			 key:   *obj.Key,
			 size:  *obj.Size,
			 mtime: *obj.LastModified,
			 isDir: false,
		 })
	 }
 
	 var nextToken string
	 if resp.NextContinuationToken != nil {
		 nextToken = *resp.NextContinuationToken
	 }
 
	 return objs, *resp.IsTruncated, nextToken, nil
 }
 
 // 复制对象
 func (r *r2Client) Copy(dst, src string) error {
	 params := &s3.CopyObjectInput{
		 Bucket:     aws.String(r.bucket),
		 Key:        aws.String(dst),
		 CopySource: aws.String(r.bucket + "/" + src),
	 }
 
	 _, err := r.s3.CopyObject(params)
	 return err
 }
 
 // R2 不支持创建 Bucket，需要手动创建
 func (r *r2Client) Create() error {
	 return fmt.Errorf("Cloudflare R2 does not support creating buckets via API")
 }
 