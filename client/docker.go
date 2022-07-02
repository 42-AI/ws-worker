package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/42-AI/ws-worker/conf"
	"github.com/42-AI/ws-worker/internal/logger"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"strings"
)

type DockerHandler struct {
	client client.APIClient
	log    *logger.Logger
	config conf.Configuration
}

func (dh *DockerHandler) new(log *logger.Logger, config conf.Configuration) error {
	dh.log = log
	dh.config = config
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		dh.log.Error("", zap.Error(err))
		return err
	}
	dh.client = cli
	return nil
}

// Run a container and returns its logs and an error if container exited with error code != 0
func (dh *DockerHandler) runImage(ctx context.Context, image string, env []string) ([]byte, error) {
	dh.log.Info("running container", zap.String("image", image))
	reader, err := dh.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		dh.log.Error("failed to pull image", zap.Error(err))
		return nil, err
	}
	buf := new(strings.Builder)
	io.Copy(buf, reader)
	str := strings.Split(buf.String(), "\n")
	for _, s := range str {
		if s != "" {
			simple := struct {
				Status string `json:"status"`
			}{}
			err := json.Unmarshal([]byte(s), &simple)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal: %w", err)
			}
			dh.log.Info("docker " + simple.Status)
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		dh.log.Error("", zap.Error(err))
		return nil, err
	}
	_ = os.Mkdir(dh.config.WS_DOCKER_LOG_FOLDER, os.ModeDir)
	volumes := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: path.Join(dir, dh.config.WS_DOCKER_LOG_FOLDER),
			Target: "/logs",
		},
	}
	resp, err := dh.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: image,
			Tty:   false,
			Env:   env,
		},
		&container.HostConfig{
			Mounts: volumes,
		},
		nil,
		nil,
		"")
	if err != nil {
		dh.log.Error("failed to create container", zap.Error(err))
		return nil, err
	}

	err = dh.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		dh.log.Error("failed to start container", zap.Error(err))
		return nil, err
	}

	statusCh, errCh := dh.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			dh.log.Error("", zap.Error(err))
			return nil, err
		}
	case <-statusCh:
	}

	out, err := dh.client.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		dh.log.Error("", zap.Error(err))
		return nil, err
	}
	buff := new(bytes.Buffer)
	//io.Copy(buff, io.LimitReader(out, dh.config.WS_MAX_LOGS_SIZE))
	io.Copy(buff, out)
	containerLogs := buff.Bytes()

	inspect, err := dh.client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		dh.log.Error("", zap.Error(err))
		return containerLogs, err
	}
	err = nil
	if inspect.State.ExitCode != 0 {
		err = fmt.Errorf("container exited with exit code %d", inspect.State.ExitCode)
	}

	return containerLogs, err
}
