package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type MinioConfig struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string
	Prefix    string
	Dir       string
}

func UploadTiles(cfg MinioConfig) error {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               cfg.Endpoint,
			SigningRegion:     cfg.Region,
			HostnameImmutable: true,
		}, nil
	})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(awsCfg)

	// Ensure the bucket exists
	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		_, err = client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(cfg.Bucket),
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", cfg.Bucket, err)
		}
		log.Printf("Created bucket: %s", cfg.Bucket)
	}

	files, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".png") {
			continue
		}

		fpath := filepath.Join(cfg.Dir, f.Name())
		file, err := os.Open(fpath)
		if err != nil {
			log.Printf("could not open file %s: %v", f.Name(), err)
			continue
		}
		defer file.Close()

		_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(cfg.Bucket),
			Key:    aws.String(filepath.Join(cfg.Prefix, f.Name())),
			Body:   file,
		})
		if err != nil {
			log.Printf("failed to upload %s: %v", f.Name(), err)
		} else {
			log.Printf("uploaded: %s", f.Name())
		}
	}

	return nil
}
