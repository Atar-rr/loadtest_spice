package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"os"
	"sync"
	"time"
)

const FileName = "data.json"

var Types = []string{TypeProject, TypeService, TypeServiceNova, TypeVM}

var RelationsByTypes = map[string][]string{
	TypeProject:     {RelationEditor, RelationViewer, RelationMember},
	TypeService:     {RelationEditor},
	TypeServiceNova: {RelationVmCreator},
	TypeVM:          {RelationEditor, RelationViewer}, //
}

type Users struct {
	rwm  sync.RWMutex
	List []uuid.UUID            `json:"list"`
	Rel  map[uuid.UUID][]RelReq `json:"rel"`
}

func initUsers() *Users {
	return &Users{
		rwm:  sync.RWMutex{},
		List: make([]uuid.UUID, 0, UsersCount),
		Rel:  make(map[uuid.UUID][]RelReq),
	}
}

func (u *Users) generate() {
	start := time.Now()

	fmt.Println("Generating users...", start.Format("2006-01-02 15:04:05"))

	wg := sync.WaitGroup{}

	wg.Add(WorkerGenerateCount)

	for i := 0; i < WorkerGenerateCount; i++ {
		go func() {
			for i := 0; i < UsersCount/WorkerGenerateCount; i++ {
				newU := uuid.New()

				u.rwm.Lock()
				u.List = append(u.List, newU)
				u.rwm.Unlock()
			}

			wg.Done()
		}()
	}

	wg.Wait()

	end := time.Since(start)
	fmt.Println("Generating users...done: ", end.String())
}

func (u *Users) makeRelations(ctx context.Context, spiceDbClient *SpiceDbClient) error {
	start := time.Now()
	fmt.Println("Making relations...: ", start.Format("2006-01-02 15:04:05"))

	// make roles
	relReqs := make([]RelReq, 0, UsersCount)

	for _, v := range u.List {
		relReq := RelReq{
			ObjectType:  TypeRole,
			ObjectID:    TypeRole + v.String(),
			Relation:    RelationMember,
			SubjectType: TypeUser,
			SubjectID:   v.String(),
		}

		relReqs = append(relReqs, relReq)
		u.Rel[v] = append(u.Rel[v], relReq)
	}

	// make projects
	pRoles := u.List[0:ProjectCount]
	projectIds := make([]uuid.UUID, 0, len(pRoles))
	for _, v := range pRoles {
		objectId := uuid.New()

		relReq := RelReq{
			ObjectType:  TypeProject,
			ObjectID:    objectId.String(),
			Relation:    RelationEditor,
			SubjectType: TypeRole,
			SubjectID:   v.String(),
		}

		projectIds = append(projectIds, objectId)

		relReqs = append(relReqs, relReq)

		u.Rel[v] = append(u.Rel[v], relReq)
	}

	// make service
	serviceIds := make([]uuid.UUID, 0, len(pRoles))
	novaIds := make([]uuid.UUID, 0, ServiceNovaCount)
	for _, v := range projectIds {

		for i := 0; i < ServiceCount; i++ {
			objectId := uuid.New()

			relReq := RelReq{
				ObjectType:  TypeService,
				ObjectID:    objectId.String(),
				Relation:    RelationParent,
				SubjectType: TypeProject,
				SubjectID:   v.String(),
			}

			relReqs = append(relReqs, relReq)

			u.Rel[v] = append(u.Rel[v], relReq)

			serviceIds = append(serviceIds, objectId)
		}

		// make NOVA
		novaIds = make([]uuid.UUID, len(pRoles))
		for i := 0; i < ServiceNovaCount; i++ {
			objectId := uuid.New()

			relReq := RelReq{
				ObjectType:  TypeService,
				ObjectID:    objectId.String(),
				Relation:    RelationParent,
				SubjectType: TypeProject,
				SubjectID:   v.String(),
			}

			relReqs = append(relReqs, relReq)

			u.Rel[v] = append(u.Rel[v], relReq)

			novaIds = append(novaIds, objectId)
		}
	}

	// make VM
	vmIds := make([]uuid.UUID, len(pRoles))
	for _, v := range novaIds {
		for i := 0; i < VMCount; i++ {
			objectId := uuid.New()

			relReq := RelReq{
				ObjectType:  TypeVM,
				ObjectID:    objectId.String(),
				Relation:    RelationParent,
				SubjectType: TypeServiceNova,
				SubjectID:   v.String(),
			}

			relReqs = append(relReqs, relReq)

			u.Rel[v] = append(u.Rel[v], relReq)
			vmIds = append(vmIds, objectId)
		}
	}

	for k, _ := range u.Rel {
		randType := u.getRandType()
		randRel := u.getRandRelation(randType)

		relReq := RelReq{
			ObjectType:  randType,
			Relation:    randRel,
			SubjectType: TypeRole,
			SubjectID:   k.String(),
		}

		switch randType {
		case TypeProject:
			relReq.ObjectID = projectIds[rand.Intn(len(projectIds))].String()
		case TypeService:
			relReq.ObjectID = serviceIds[rand.Intn(len(serviceIds))].String()
		case TypeVM:
			relReq.ObjectID = vmIds[rand.Intn(len(vmIds))].String()
		case TypeServiceNova:
			relReq.ObjectID = novaIds[rand.Intn(len(novaIds))].String()
		}

		relReqs = append(relReqs, relReq)

		u.Rel[k] = append(u.Rel[k], relReq)
	}

	dataUsers, _ := json.Marshal(u)

	_ = os.Remove(FileName)

	file, err := os.Create(FileName)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.Write(dataUsers)
	if err != nil {
		return err
	}

	err = spiceDbClient.AddRelationships(ctx, relReqs)
	if err != nil {
		return err
	}

	end := time.Since(start)
	fmt.Println("Make relations....done: ", end.String())
	return nil
}

func (u *Users) getRandType() string {
	return Types[rand.Intn(len(Types))]
}

func (u *Users) getRandRelation(typeName string) string {
	relType := RelationsByTypes[typeName]

	return relType[rand.Intn(len(relType))]
}
