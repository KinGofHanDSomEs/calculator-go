package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	task "github.com/kingofhandsomes/calculator-go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Agent struct {
	db                  *sql.DB
	grpc_port           string
	timeAdditon         time.Duration
	timeSubtraction     time.Duration
	timeMultiplications time.Duration
	timeDivisions       time.Duration
	workers             int
}

func New(db *sql.DB, grpc_port int, ta, ts, tm, td time.Duration, workers int) *Agent {
	return &Agent{
		db:                  db,
		grpc_port:           fmt.Sprint(grpc_port),
		timeAdditon:         ta,
		timeSubtraction:     ts,
		timeMultiplications: tm,
		timeDivisions:       td,
		workers:             workers,
	}
}

func (a *Agent) MustRun() {
	const op = "agent.MustRun"

	a.db.Exec("UPDATE tasks SET stat = 'ready' WHERE stat = 'in progress'")

	var wg sync.WaitGroup
	var mu sync.Mutex

	conn, err := grpc.Dial(fmt.Sprintf("localhost:%s", a.grpc_port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic("error connecting to grpc")
	}
	defer conn.Close()
	client := task.NewTaskServiceClient(conn)

	for i := 0; i < a.workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				mu.Lock()
				tsk, err := client.GetTask(context.TODO(), &task.GetTaskRequest{})
				mu.Unlock()
				if err != nil {
					continue
				}

				log.Printf("%s: get task, goroutine: %d, login: %s, expression: %d, task: %d, arg1: %f, arg2: %f, operation: %s\n", op, i, tsk.GetLogin(), tsk.GetIdExpression(), tsk.GetIdTask(), tsk.GetArg1(), tsk.GetArg2(), tsk.GetOperation())

				res, duration := a.work(tsk.GetArg1(), tsk.GetArg2(), tsk.GetOperation())

				log.Printf("%s: post task, goroutine: %d, login: %s, expression: %d, task: %d, operation time: %d, result: %f\n", op, i, tsk.GetLogin(), tsk.GetIdExpression(), tsk.GetIdTask(), duration, res)

				client.PostTask(context.TODO(), &task.PostTaskRequest{
					Login:         tsk.GetLogin(),
					IdExpression:  tsk.GetIdExpression(),
					IdTask:        tsk.GetIdTask(),
					OperationTime: int64(duration),
					Result:        res,
				})
			}
		}(i + 1)
	}
	wg.Wait()
}

func (a *Agent) work(arg1, arg2 float32, oper string) (float32, time.Duration) {
	switch oper {
	case "+":
		<-time.After(a.timeAdditon)
		return arg1 + arg2, a.timeAdditon
	case "-":
		<-time.After(a.timeSubtraction)
		return arg1 - arg2, a.timeSubtraction
	case "*":
		<-time.After(a.timeMultiplications)
		return arg1 * arg2, a.timeMultiplications
	default:
		<-time.After(a.timeDivisions)
		return arg1 / arg2, a.timeDivisions
	}
}
