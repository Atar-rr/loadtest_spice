package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
)

const (
	UsersCount          = 100000
	WorkerGenerateCount = 10

	ProjectCount     = UsersCount / 1000
	ServiceCount     = ProjectCount / 100
	ServiceNovaCount = ProjectCount / 100
	VMCount          = ServiceNovaCount * 10

	GrpcPresharedKey = "foobar"

	SpicedbAddress    = "127.0.0.1:50051"
	SpicedbSchemaPath = "./testdata/schema.zed"

	TypeProject     = "project"
	TypeService     = "service"
	TypeServiceNova = "service_nova"
	TypeVM          = "virtual_machine"
	TypeRole        = "role"
	TypeUser        = "user"

	RelationMember    = "member"
	RelationEditor    = "editor"
	RelationViewer    = "viewer"
	RelationParent    = "parent"
	RelationVmCreator = "vm_creator"
)

type CheckReq struct {
	ObjectType  string
	ObjectID    string
	SubjectType string
	SubjectID   string
	Permission  string
}

type RelReq struct {
	ObjectType  string
	ObjectID    string
	SubjectType string
	SubjectID   string
	Relation    string
}

func loadTest(ctx context.Context, client *SpiceDbClient, users *Users, concurrency int, timer time.Duration) {
	mCh := make(chan Metric, concurrency)
	readEnd := make(chan bool)

	ctx, cancel := context.WithTimeout(ctx, time.Minute*timer)

	stat := InitStat()

	go stat.readMetrics(ctx, mCh, readEnd)

	wg := sync.WaitGroup{}

	wg.Add(concurrency)

	rand.Shuffle(len(users.list), func(i, j int) {
		users.list[i], users.list[j] = users.list[j], users.list[i]
	})

	usersCheck := users.list[0 : UsersCount/10]

	defer cancel()
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					reqs := users.rel[usersCheck[rand.Intn(len(usersCheck))]]
					if len(reqs) > 1 {
						t := 1
						_ = t
					}
					req := reqs[rand.Intn(len(reqs))]

					start := time.Now()

					_, err := client.CheckPermission(context.Background(), &CheckReq{
						ObjectType:  req.ObjectType,
						ObjectID:    req.ObjectID,
						Permission:  req.Relation,
						SubjectType: req.SubjectType,
						SubjectID:   req.SubjectID,
					})

					metric := Metric{}
					if err != nil {
						metric.errResp = true
					}

					metric.duration = int(time.Since(start).Nanoseconds())
					mCh <- metric
				}
			}
		}()
	}

	wg.Wait()

	close(mCh)

	<-readEnd

	stat.calculate(timer)
}

/*
*
- 1 команда с опциями
- - init (создаем сущности)
- - rps (n int)
- - time (min)
*/
func main() {
	var concurrency int
	var timer int

	flag.IntVar(&concurrency, "rps", 100000, "a int var")
	flag.IntVar(&timer, "timer", 1, "a int var")
	flag.Parse()

	ctx := context.Background()

	client, err := InitClient()
	if err != nil {
		log.Fatal(err)
	}

	//if len(os.Args) > 1 && os.Args[1] == "init" {
	data, err := os.ReadFile(SpicedbSchemaPath)
	if err != nil {
		log.Fatal(err)
	}

	err = client.WriteSchema(ctx, data)
	if err != nil {
		log.Fatalf("failed to write schema: %s", err)
	}

	users := initUsers()
	users.generate()
	err = users.makeRelations(ctx, client)

	if err != nil {
		log.Fatalf("make relation: %s", err)
	}
	//}

	loadTest(ctx, client, users, concurrency, time.Duration(timer))
}
