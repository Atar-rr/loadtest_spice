package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
)

const (
	UsersCount          = 1000000
	WorkerGenerateCount = 1000

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
	ObjectType  string `json:"object_type"`
	ObjectID    string `json:"object_id"`
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	Relation    string `json:"relation"`
}

func loadTest(ctx context.Context, client *SpiceDbClient, concurrency int, timer time.Duration) error {
	mCh := make(chan Metric, concurrency)
	readEnd := make(chan bool)

	file, err := os.OpenFile(FileName, os.O_RDONLY, 0666)
	defer file.Close()
	if err != nil {
		return err
	}
	b, err := io.ReadAll(file)

	var users Users
	err = json.Unmarshal(b, &users)
	if err != nil {
		return err
	}

	rand.Shuffle(len(users.List), func(i, j int) {
		users.List[i], users.List[j] = users.List[j], users.List[i]
	})

	usersCheck := users.List[0 : UsersCount/10]

	stat := InitStat()

	ctx, cancel := context.WithTimeout(ctx, time.Minute*timer)
	go stat.readMetrics(ctx, mCh, readEnd)

	wg := sync.WaitGroup{}

	wg.Add(concurrency)

	defer cancel()
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					reqs := users.Rel[usersCheck[rand.Intn(len(usersCheck))]]
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

	return nil
}

/*
*
- 1 команда с опциями
- - init (создаем сущности)
- - rps (n int)
- - time (min)
*/
func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU() * 2)

	var concurrency int
	var timer int
	var init bool

	flag.IntVar(&concurrency, "rps", 100000, "a int var")
	flag.IntVar(&timer, "timer", 1, "a int var")
	flag.BoolVar(&init, "init", false, "a bool var")
	flag.Parse()

	ctx := context.Background()

	client, err := InitClient()
	if err != nil {
		log.Fatal(err)
	}

	if init {
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
	}

	err = loadTest(ctx, client, concurrency, time.Duration(timer))
	if err != nil {
		log.Fatal(err)
	}
}
