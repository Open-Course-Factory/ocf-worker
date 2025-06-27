package garage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"ocf-worker/pkg/storage"
)

type garageStorage struct {
	client *s3.Client
	bucket string
}

// NewGarageStorage crée une nouvelle instance de storage Garage S3-compatible
func NewGarageStorage(cfg *storage.StorageConfig) (storage.Storage, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("garage endpoint is required")
	}
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("garage access key is required")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("garage secret key is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("garage bucket is required")
	}

	// Configuration AWS SDK v2 pour Garage
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"", // session token (pas nécessaire pour Garage)
		)),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Créer le client S3 avec endpoint personnalisé pour Garage
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true // Important pour Garage/MinIO
	})

	garage := &garageStorage{
		client: client,
		bucket: cfg.Bucket,
	}

	// Vérifier que le bucket existe (optionnel)
	if err := garage.ensureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	return garage, nil
}

// ensureBucket vérifie que le bucket existe et le crée si nécessaire
func (g *garageStorage) ensureBucket(ctx context.Context) error {
	// Vérifier si le bucket existe
	_, err := g.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(g.bucket),
	})
	if err == nil {
		return nil // Le bucket existe
	}

	// Essayer de créer le bucket s'il n'existe pas
	_, createErr := g.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(g.bucket),
	})
	if createErr != nil {
		return fmt.Errorf("bucket %s does not exist and cannot be created: %w", g.bucket, createErr)
	}

	return nil
}

func (g *garageStorage) Upload(ctx context.Context, path string, data io.Reader) error {
	// S3 utilise des clés sans "/" initial
	key := strings.TrimPrefix(path, "/")

	_, err := g.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(g.bucket),
		Key:    aws.String(key),
		Body:   data,
		// Définir le content-type basé sur l'extension
		ContentType: aws.String(getContentType(key)),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object %s to bucket %s: %w", key, g.bucket, err)
	}

	return nil
}

func (g *garageStorage) Download(ctx context.Context, path string) (io.Reader, error) {
	key := strings.TrimPrefix(path, "/")

	result, err := g.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(g.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download object %s from bucket %s: %w", key, g.bucket, err)
	}

	return result.Body, nil
}

func (g *garageStorage) Exists(ctx context.Context, path string) (bool, error) {
	key := strings.TrimPrefix(path, "/")

	_, err := g.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(g.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Vérifier si c'est une erreur "not found"
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence %s: %w", key, err)
	}

	return true, nil
}

func (g *garageStorage) Delete(ctx context.Context, path string) error {
	key := strings.TrimPrefix(path, "/")

	_, err := g.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(g.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %w", key, g.bucket, err)
	}

	return nil
}

func (g *garageStorage) List(ctx context.Context, prefix string) ([]string, error) {
	// S3 utilise des préfixes sans "/" initial
	cleanPrefix := strings.TrimPrefix(prefix, "/")

	var objects []string
	paginator := s3.NewListObjectsV2Paginator(g.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(g.bucket),
		Prefix: aws.String(cleanPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects with prefix %s: %w", cleanPrefix, err)
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				objects = append(objects, *obj.Key)
			}
		}
	}

	return objects, nil
}

func (g *garageStorage) GetURL(ctx context.Context, path string) (string, error) {
	key := strings.TrimPrefix(path, "/")

	// Générer une URL présignée valide pour 1 heure
	presigner := s3.NewPresignClient(g.client)

	request, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(g.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 3600 // 1 heure
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL for %s: %w", key, err)
	}

	return request.URL, nil
}

// getContentType détermine le content-type basé sur l'extension du fichier
func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md":
		return "text/markdown"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".html":
		return "text/html"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}
