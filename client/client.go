package client

import (
	"context"
	"crypto/tls"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"time"

	pb "github.com/42-AI/ws-worker/proto"
)

func Run() {
	lg, cf, dh, err := dependencies()
	if err != nil {
		panic(err)
	}

	var creds credentials.TransportCredentials
	if cf.WS_SERVER_CERT_FILE == "" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		creds = credentials.NewTLS(tlsConfig)
		lg.Warn("no server cert file provided, grpc server authenticity can't be checked")
	} else {
		creds, err = credentials.NewClientTLSFromFile(cf.WS_SERVER_CERT_FILE, "")
		if err != nil {
			lg.Panic("failed to load server cert", zap.Error(err))
		}
	}
	address := cf.WS_GRPC_HOST + ":" + cf.WS_GRPC_PORT
	lg.Info("connecting to grpc server", zap.String("address", address))
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(creds), grpc.WithBlock())
	if err != nil {
		lg.Panic("failed to connect", zap.Error(err))
	}
	defer conn.Close()
	lg.Info("connection acquired")
	c := pb.NewApiClient(conn)

	ctx := context.Background()
	lg.Info("pulling new task...", zap.Duration("sleep", cf.WS_SLEEP_BETWEEN_CALL))
	for {
		r, err := c.StartTask(ctx, &pb.StartTaskReq{WithGPU: true})
		if getErrorCode(err) == getErrorCode(errNoTasksInQueue) {
			time.Sleep(cf.WS_SLEEP_BETWEEN_CALL)
			continue
		} else if err != nil {
			lg.Error("failed to start task", zap.Error(err))
			time.Sleep(cf.WS_SLEEP_BETWEEN_CALL)
			continue
		}
		lg.Info("starting task", zap.String("id", r.TaskId))
		containerLogs, err := dh.runImage(ctx, r.Job.DockerImage, r.Job.Env)
		var errString string
		if err != nil {
			lg.Error("container run failed", zap.Error(err))
			errString = err.Error()
		}
		_, err = c.EndTask(ctx, &pb.EndTaskReq{TaskId: r.TaskId, Error: errString, Logs: containerLogs})
		if err != nil {
			lg.Error("failed to end task", zap.Error(err))
		} else {
			lg.Info("task ended", zap.String("id", r.TaskId))
		}
	}
}
