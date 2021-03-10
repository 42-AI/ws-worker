package client

import (
	"context"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"time"

	pb "github.com/jjauzion/ws-worker/proto"
)

const (
	sleepBetweenCall = 30 * time.Second
)

func Run() {
	lg, cf, err := dependencies()
	if err != nil {
		log.Panic(err)
	}
	address := cf.WS_GRPC_HOST + ":" + cf.WS_GRPC_PORT
	lg.Info("connecting to grpc server", zap.String("address", address))
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		lg.Panic("failed to connect", zap.Error(err))
	}
	defer conn.Close()
	lg.Info("connection acquired")
	c := pb.NewApiClient(conn)

	ctx := context.Background()
	lg.Info("pulling new task...", zap.Duration("sleep", sleepBetweenCall))
	for {
		r, err := c.StartTask(ctx, &pb.StartTaskReq{WithGPU: true})
		if getErrorCode(err) == getErrorCode(errNoTasksInQueue) {
			time.Sleep(sleepBetweenCall)
			continue
		} else if err != nil {
			lg.Error("failed to start task", zap.Error(err))
			return
		}
		lg.Info("start task image", zap.String("image", r.Job.DockerImage), zap.String("dataset", r.Job.Dataset))
	}
}
