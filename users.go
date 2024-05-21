package main

import (
	"context"
	"github.com/google/uuid"
	"math/rand"
	"sync"
)

var Types = []string{TypeProject, TypeService, TypeServiceNova, TypeVM}

var RelationsByTypes = map[string][]string{
	TypeProject:     {RelationEditor, RelationViewer, RelationMember},
	TypeService:     {RelationEditor},
	TypeServiceNova: {RelationVmCreator},
	TypeVM:          {RelationEditor, RelationViewer}, //
}

type Users struct {
	rwm  sync.RWMutex
	list []uuid.UUID
	rel  map[uuid.UUID][]RelReq
}

func initUsers() *Users {
	return &Users{
		rwm:  sync.RWMutex{},
		list: make([]uuid.UUID, 0, UsersCount),
		rel:  make(map[uuid.UUID][]RelReq),
	}
}

func (u *Users) generate() {
	wg := sync.WaitGroup{}

	wg.Add(WorkerGenerateCount)

	for i := 0; i < WorkerGenerateCount; i++ {
		go func() {
			for i := 0; i < UsersCount/WorkerGenerateCount; i++ {
				newU := uuid.New()

				u.rwm.Lock()
				u.list = append(u.list, newU)
				u.rwm.Unlock()
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

func (u *Users) makeRelations(ctx context.Context, spiceDbClient *SpiceDbClient) error {
	// make roles
	relReqs := make([]RelReq, 0, UsersCount)

	for _, v := range u.list {
		relReq := RelReq{
			ObjectType:  TypeRole,
			ObjectID:    TypeRole + v.String(),
			Relation:    RelationMember,
			SubjectType: TypeUser,
			SubjectID:   v.String(),
		}

		relReqs = append(relReqs, relReq)
		u.rel[v] = append(u.rel[v], relReq)
	}

	// make projects
	pRoles := u.list[0:ProjectCount]
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

		u.rel[v] = append(u.rel[v], relReq)
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

			u.rel[v] = append(u.rel[v], relReq)

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

			u.rel[v] = append(u.rel[v], relReq)

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

			u.rel[v] = append(u.rel[v], relReq)
			vmIds = append(vmIds, objectId)
		}
	}

	for k, _ := range u.rel {
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

		u.rel[k] = append(u.rel[k], relReq)
	}

	err := spiceDbClient.AddRelationships(ctx, relReqs)
	if err != nil {
		return err
	}

	return nil
}

func (u *Users) getRandType() string {
	return Types[rand.Intn(len(Types))]
}

func (u *Users) getRandRelation(typeName string) string {
	relType := RelationsByTypes[typeName]

	return relType[rand.Intn(len(relType))]
}
