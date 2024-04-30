package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/offen/docker-volume-backup/internal/errwrap"
)

func runProxy(c *Config) (err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return errwrap.Wrap(err, "failed to create docker client")
	}
	defer cli.Close()

	reader, err := cli.ImagePull(context.Background(), "offen/docker-volume-backup:v2.39.1", types.ImagePullOptions{})
	if err != nil {
		return errwrap.Wrap(err, "unable to pull image")
	}
	io.Copy(os.Stdout, reader)
	defer reader.Close()

	networkResp, err := cli.NetworkList(context.Background(), types.NetworkListOptions{Filters: filters.NewArgs(filters.KeyValuePair{Key: "name", Value: "volumes_default"})})

	resp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image:      "offen/docker-volume-backup:v2.39.1",
		Tty:        true,
		Entrypoint: []string{"backup"},
		Env: []string{
			fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", c.AwsAccessKeyID),
			fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", c.AwsSecretAccessKey),
			fmt.Sprintf("AWS_ENDPOINT=%s", c.AwsEndpoint),
			fmt.Sprintf("AWS_ENDPOINT_PROTO=%s", c.AwsEndpointProto),
			fmt.Sprintf("AWS_S3_BUCKET_NAME=%s", c.AwsS3BucketName),
			fmt.Sprintf("BACKUP_FILENAME_EXPAND=%t", c.BackupFilenameExpand),
			fmt.Sprintf("BACKUP_FILENAME=%s", c.BackupFilename),
			fmt.Sprintf("BACKUP_CRON_EXPRESSION=%s", c.BackupCronExpression),
			fmt.Sprintf("BACKUP_RETENTION_DAYS=%x", c.BackupRetentionDays),
			fmt.Sprintf("BACKUP_PRUNING_LEEWAY=%s", c.BackupPruningLeeway),
			fmt.Sprintf("BACKUP_PRUNING_PREFIX=%s", c.BackupPruningPrefix),
			fmt.Sprintf("HOSTNAME=%s", os.Getenv("HOSTNAME")),
		},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: c.source,
				Target: "/backup",
			},
			{
				Type:   mount.TypeBind,
				Source: "/var/run/docker.sock",
				Target: "/var/run/docker.sock",
			},
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"mynetwork": {
				NetworkID: networkResp[0].ID,
			},
		},
	}, nil, "")
	if err != nil {
		return errwrap.Wrap(err, "unable to create container")
	}

	if err := cli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		return errwrap.Wrap(err, "unable to start container")
	}

	statusCh, errCh := cli.ContainerWait(context.Background(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return errwrap.Wrap(err, "error running container")
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(context.Background(), resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return errwrap.Wrap(err, "unable to get logs from container")
	}

	err = cli.ContainerRemove(context.Background(), resp.ID, types.ContainerRemoveOptions{})
	if err != nil {
		return errwrap.Wrap(err, "unable to remove container")
	}

	io.Copy(os.Stdout, out)
	defer out.Close()

	return nil
}
