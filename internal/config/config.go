// Copyright 2022 - offen.software <hioffen@posteo.de>
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/offen/docker-volume-backup/internal/errwrap"
	"github.com/offen/docker-volume-backup/internal/storage/azure"
	"github.com/offen/docker-volume-backup/internal/storage/dropbox"
	"github.com/offen/docker-volume-backup/internal/storage/s3"
	"github.com/offen/docker-volume-backup/internal/storage/ssh"
	"github.com/offen/docker-volume-backup/internal/storage/webdav"
)

type StorageConfig struct {
	AWS *s3.Config
	//	AwsS3BucketName             string          `split_words:"true"`
	//	AwsS3Path                   string          `split_words:"true"`
	//	AwsEndpoint                 string          `split_words:"true" default:"s3.amazonaws.com"`
	//	AwsEndpointProto            string          `split_words:"true" default:"https"`
	//	AwsEndpointInsecure         bool            `split_words:"true"`
	//	AwsEndpointCACert           CertDecoder     `envconfig:"AWS_ENDPOINT_CA_CERT"`
	//	AwsStorageClass             string          `split_words:"true"`
	//	AwsAccessKeyID              string          `envconfig:"AWS_ACCESS_KEY_ID"`
	//	AwsSecretAccessKey          string          `split_words:"true"`
	//	AwsIamRoleEndpoint          string          `split_words:"true"`
	//	AwsPartSize                 int64           `split_words:"true"`
	Webdav *webdav.Config
	//	WebdavUrl                   string          `split_words:"true"`
	//	WebdavUrlInsecure           bool            `split_words:"true"`
	//	WebdavPath                  string          `split_words:"true" default:"/"`
	//	WebdavUsername              string          `split_words:"true"`
	//	WebdavPassword              string          `split_words:"true"`
	SSH *ssh.Config
	//	SSHHostName                 string          `split_words:"true"`
	//	SSHPort                     string          `split_words:"true" default:"22"`
	//	SSHUser                     string          `split_words:"true"`
	//	SSHPassword                 string          `split_words:"true"`
	//	SSHIdentityFile             string          `split_words:"true" default:"/root/.ssh/id_rsa"`
	//	SSHIdentityPassphrase       string          `split_words:"true"`
	//	SSHRemotePath               string          `split_words:"true"`
	Azure *azure.Config
	//	AzureStorageAccountName       string          `split_words:"true"`
	//	AzureStoragePrimaryAccountKey string          `split_words:"true"`
	//	AzureStorageConnectionString  string          `split_words:"true"`
	//	AzureStorageContainerName     string          `split_words:"true"`
	//	AzureStoragePath              string          `split_words:"true"`
	//	AzureStorageEndpoint          string          `split_words:"true" default:"https://{{ .AccountName }}.blob.core.windows.net/"`
	Dropbox *dropbox.Config
	//	DropboxEndpoint         string        `split_words:"true" default:"https://api.dropbox.com/"`
	//	DropboxOAuth2Endpoint   string        `envconfig:"DROPBOX_OAUTH2_ENDPOINT" default:"https://api.dropbox.com/"`
	//	DropboxRefreshToken     string        `split_words:"true"`
	//	DropboxAppKey           string        `split_words:"true"`
	//	DropboxAppSecret        string        `split_words:"true"`
	//	DropboxRemotePath       string        `split_words:"true"`
	//	DropboxConcurrencyLevel NaturalNumber `split_words:"true" default:"6"`
}

type EmailNotification struct {
	EmailNotificationRecipient string `split_words:"true"`
	EmailNotificationSender    string `split_words:"true" default:"noreply@nohost"`
	EmailSMTPHost              string `envconfig:"EMAIL_SMTP_HOST"`
	EmailSMTPPort              int    `envconfig:"EMAIL_SMTP_PORT" default:"587"`
	EmailSMTPUsername          string `envconfig:"EMAIL_SMTP_USERNAME"`
	EmailSMTPPassword          string `envconfig:"EMAIL_SMTP_PASSWORD"`
}

type NotificationConfig struct {
	NotificationURLs []string           `envconfig:"NOTIFICATION_URLS"`
	Level            string             `split_words:"true" default:"error"`
	Email            *EmailNotification `label:"allowEmpty"`
}

type BackupConfig struct {
	GzipParallelism       WholeNumber     `split_words:"true" default:"1"`
	Compression           CompressionType `split_words:"true" default:"gz"`
	Sources               string          `split_words:"true" default:"/backup"`
	Filename              string          `split_words:"true" default:"backup-%Y-%m-%dT%H-%M-%S.{{ .Extension }}"`
	FilenameExpand        bool            `split_words:"true"`
	LatestSymlink         string          `split_words:"true"`
	Archive               string          `split_words:"true" default:"/archive"`
	CronExpression        string          `split_words:"true" default:"@daily"`
	RetentionDays         int32           `split_words:"true" default:"-1"`
	PruningLeeway         time.Duration   `split_words:"true" default:"1m"`
	PruningPrefix         string          `split_words:"true"`
	StopContainerLabel    string          `split_words:"true"`
	StopDuringBackupLabel string          `split_words:"true" default:"true"`
	StopServiceTimeout    time.Duration   `split_words:"true" default:"5m"`
	FromSnapshot          bool            `split_words:"true"`
	ExcludeRegexp         RegexpDecoder   `split_words:"true"`
	SkipBackendsFromPrune []string        `split_words:"true"`

	ExecLabel         string        `split_words:"true"`
	ExecForwardOutput bool          `split_words:"true"`
	LockTimeout       time.Duration `split_words:"true" default:"60m"`
	GpgPassphrase     string        `split_words:"true"`
}

type Config struct {
	Source       string
	Backup       BackupConfig
	Notification NotificationConfig
	Storage      StorageConfig
}

func (c *Config) SetDefaults() {
	c.Backup.SetDefaults()
	c.Notification.SetDefaults()
}

func (c *BackupConfig) SetDefaults() {
	c.GzipParallelism = 1
	c.Compression = "gz"
	c.Sources = "/backup"
	c.Filename = "backup-%Y-%m-%dT%H-%M-%S.{{ .Extension }}"
	c.Archive = "/archive"
	c.CronExpression = "@daily"
	c.RetentionDays = -1
	c.PruningLeeway = 1 * time.Minute
	c.StopDuringBackupLabel = "true"
	c.StopServiceTimeout = 5 * time.Minute
	c.LockTimeout = 60 * time.Minute
}

func (c *NotificationConfig) SetDefaults() {
	c.Level = "error"
}

type CompressionType string

func (c *CompressionType) Decode(v string) error {
	switch v {
	case "gz", "zst":
		*c = CompressionType(v)
		return nil
	default:
		return errwrap.Wrap(nil, fmt.Sprintf("error decoding compression type %s", v))
	}
}

func (c *CompressionType) String() string {
	return string(*c)
}

type CertDecoder struct {
	Cert *x509.Certificate
}

func (c *CertDecoder) Decode(v string) error {
	if v == "" {
		return nil
	}
	content, err := os.ReadFile(v)
	if err != nil {
		content = []byte(v)
	}
	block, _ := pem.Decode(content)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errwrap.Wrap(err, "error parsing certificate")
	}
	*c = CertDecoder{Cert: cert}
	return nil
}

type RegexpDecoder struct {
	Re *regexp.Regexp
}

func (r *RegexpDecoder) Decode(v string) error {
	if v == "" {
		return nil
	}
	re, err := regexp.Compile(v)
	if err != nil {
		return errwrap.Wrap(err, fmt.Sprintf("error compiling given regexp `%s`", v))
	}
	*r = RegexpDecoder{Re: re}
	return nil
}

// NaturalNumber is a type that can be used to decode a positive, non-zero natural number
type NaturalNumber int

func (n *NaturalNumber) Decode(v string) error {
	asInt, err := strconv.Atoi(v)
	if err != nil {
		return errwrap.Wrap(nil, fmt.Sprintf("error converting %s to int", v))
	}
	if asInt <= 0 {
		return errwrap.Wrap(nil, fmt.Sprintf("expected a natural number, got %d", asInt))
	}
	*n = NaturalNumber(asInt)
	return nil
}

func (n *NaturalNumber) Int() int {
	return int(*n)
}

// WholeNumber is a type that can be used to decode a positive whole number, including zero
type WholeNumber int

func (n *WholeNumber) Decode(v string) error {
	asInt, err := strconv.Atoi(v)
	if err != nil {
		return errwrap.Wrap(nil, fmt.Sprintf("error converting %s to int", v))
	}
	if asInt < 0 {
		return errwrap.Wrap(nil, fmt.Sprintf("expected a whole, positive number, including zero. Got %d", asInt))
	}
	*n = WholeNumber(asInt)
	return nil
}

func (n *WholeNumber) Int() int {
	return int(*n)
}
