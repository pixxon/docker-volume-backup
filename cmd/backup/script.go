// Copyright 2022 - offen.software <hioffen@posteo.de>
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/offen/docker-volume-backup/internal/config"
	"github.com/offen/docker-volume-backup/internal/errwrap"
	"github.com/offen/docker-volume-backup/internal/storage"
	"github.com/offen/docker-volume-backup/internal/storage/azure"
	"github.com/offen/docker-volume-backup/internal/storage/dropbox"
	"github.com/offen/docker-volume-backup/internal/storage/local"
	"github.com/offen/docker-volume-backup/internal/storage/s3"
	"github.com/offen/docker-volume-backup/internal/storage/ssh"
	"github.com/offen/docker-volume-backup/internal/storage/webdav"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/docker/docker/client"
	"github.com/leekchan/timeutil"
)

// script holds all the stateful information required to orchestrate a
// single backup run.
type script struct {
	cli       *client.Client
	storages  []storage.Backend
	logger    *slog.Logger
	sender    *router.ServiceRouter
	template  *template.Template
	hooks     []hook
	hookLevel hookLevel

	file  string
	stats *Stats

	encounteredLock bool

	c *config.Config
}

// newScript creates all resources needed for the script to perform actions against
// remote resources like the Docker engine or remote storage locations. All
// reading from env vars or other configuration sources is expected to happen
// in this method.
func newScript(c *config.Config) *script {
	stdOut, logBuffer := buffer(os.Stdout)
	return &script{
		c:      c,
		logger: slog.New(slog.NewTextHandler(stdOut, nil)),
		stats: &Stats{
			StartTime: time.Now(),
			LogOutput: logBuffer,
			Storages: map[string]StorageStats{
				"S3":      {},
				"WebDAV":  {},
				"SSH":     {},
				"Local":   {},
				"Azure":   {},
				"Dropbox": {},
			},
		},
	}
}

func (s *script) init() error {
	s.registerHook(hookLevelPlumbing, func(error) error {
		s.stats.EndTime = time.Now()
		s.stats.TookTime = s.stats.EndTime.Sub(s.stats.StartTime)
		return nil
	})

	s.file = path.Join("/tmp", s.c.Backup.Filename)

	tmplFileName, tErr := template.New("extension").Parse(s.file)
	if tErr != nil {
		return errwrap.Wrap(tErr, "unable to parse backup file extension template")
	}

	var bf bytes.Buffer
	if tErr := tmplFileName.Execute(&bf, map[string]string{
		"Extension": fmt.Sprintf("tar.%s", s.c.Backup.Compression),
	}); tErr != nil {
		return errwrap.Wrap(tErr, "error executing backup file extension template")
	}
	s.file = bf.String()

	if s.c.Backup.FilenameExpand {
		s.file = os.ExpandEnv(s.file)
		s.c.Backup.LatestSymlink = os.ExpandEnv(s.c.Backup.LatestSymlink)
		s.c.Backup.PruningPrefix = os.ExpandEnv(s.c.Backup.PruningPrefix)
	}
	s.file = timeutil.Strftime(&s.stats.StartTime, s.file)

	_, err := os.Stat("/var/run/docker.sock")
	_, dockerHostSet := os.LookupEnv("DOCKER_HOST")
	if !os.IsNotExist(err) || dockerHostSet {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return errwrap.Wrap(err, "failed to create docker client")
		}
		s.cli = cli
		s.registerHook(hookLevelPlumbing, func(err error) error {
			if err := s.cli.Close(); err != nil {
				return errwrap.Wrap(err, "failed to close docker client")
			}
			return nil
		})
	}

	logFunc := func(logType storage.LogLevel, context string, msg string, params ...any) {
		switch logType {
		case storage.LogLevelWarning:
			s.logger.Warn(fmt.Sprintf(msg, params...), "storage", context)
		default:
			s.logger.Info(fmt.Sprintf(msg, params...), "storage", context)
		}
	}

	if s.c.Storage.AWS != nil {
		s3Backend, err := s3.NewStorageBackend(*s.c.Storage.AWS, logFunc)
		if err != nil {
			return errwrap.Wrap(err, "error creating s3 storage backend")
		}
		s.storages = append(s.storages, s3Backend)
	}

	if s.c.Storage.Webdav != nil {
		webdavBackend, err := webdav.NewStorageBackend(*s.c.Storage.Webdav, logFunc)
		if err != nil {
			return errwrap.Wrap(err, "error creating webdav storage backend")
		}
		s.storages = append(s.storages, webdavBackend)
	}

	if s.c.Storage.SSH != nil {
		sshBackend, err := ssh.NewStorageBackend(*s.c.Storage.SSH, logFunc)
		if err != nil {
			return errwrap.Wrap(err, "error creating ssh storage backend")
		}
		s.storages = append(s.storages, sshBackend)
	}

	if _, err := os.Stat(s.c.Backup.Archive); !os.IsNotExist(err) {
		localConfig := local.Config{
			ArchivePath:   s.c.Backup.Archive,
			LatestSymlink: s.c.Backup.LatestSymlink,
		}
		localBackend := local.NewStorageBackend(localConfig, logFunc)
		s.storages = append(s.storages, localBackend)
	}

	if s.c.Storage.Azure != nil {
		azureBackend, err := azure.NewStorageBackend(*s.c.Storage.Azure, logFunc)
		if err != nil {
			return errwrap.Wrap(err, "error creating azure storage backend")
		}
		s.storages = append(s.storages, azureBackend)
	}

	if s.c.Storage.Dropbox != nil {
		dropboxBackend, err := dropbox.NewStorageBackend(*s.c.Storage.Dropbox, logFunc)
		if err != nil {
			return errwrap.Wrap(err, "error creating dropbox storage backend")
		}
		s.storages = append(s.storages, dropboxBackend)
	}

	if s.c.Notification.Email != nil {
		emailURL := fmt.Sprintf(
			"smtp://%s:%s@%s:%d/?from=%s&to=%s",
			s.c.Notification.Email.EmailSMTPUsername,
			s.c.Notification.Email.EmailSMTPPassword,
			s.c.Notification.Email.EmailSMTPHost,
			s.c.Notification.Email.EmailSMTPPort,
			s.c.Notification.Email.EmailNotificationSender,
			s.c.Notification.Email.EmailNotificationRecipient,
		)
		s.c.Notification.NotificationURLs = append(s.c.Notification.NotificationURLs, emailURL)
		s.logger.Warn(
			"Using EMAIL_* keys for providing notification configuration has been deprecated and will be removed in the next major version.",
		)
		s.logger.Warn(
			"Please use NOTIFICATION_URLS instead. Refer to the README for an upgrade guide.",
		)
	}

	hookLevel, ok := hookLevels[s.c.Notification.Level]
	if !ok {
		return errwrap.Wrap(nil, fmt.Sprintf("unknown NOTIFICATION_LEVEL %s", s.c.Notification.Level))
	}
	s.hookLevel = hookLevel

	if len(s.c.Notification.NotificationURLs) > 0 {
		sender, senderErr := shoutrrr.CreateSender(s.c.Notification.NotificationURLs...)
		if senderErr != nil {
			return errwrap.Wrap(senderErr, "error creating sender")
		}
		s.sender = sender

		tmpl := template.New("")
		tmpl.Funcs(templateHelpers)
		tmpl, err = tmpl.Parse(defaultNotifications)
		if err != nil {
			return errwrap.Wrap(err, "unable to parse default notifications templates")
		}

		if fi, err := os.Stat("/etc/dockervolumebackup/notifications.d"); err == nil && fi.IsDir() {
			tmpl, err = tmpl.ParseGlob("/etc/dockervolumebackup/notifications.d/*.*")
			if err != nil {
				return errwrap.Wrap(err, "unable to parse user defined notifications templates")
			}
		}
		s.template = tmpl

		// To prevent duplicate notifications, ensure the regsistered callbacks
		// run mutually exclusive.
		s.registerHook(hookLevelError, func(err error) error {
			if err == nil {
				return nil
			}
			return s.notifyFailure(err)
		})
		s.registerHook(hookLevelInfo, func(err error) error {
			if err != nil {
				return nil
			}
			return s.notifySuccess()
		})
	}

	return nil
}
