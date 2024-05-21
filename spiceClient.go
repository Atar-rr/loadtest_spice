package main

import (
	"context"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const BatchSize = 1000

type SpiceDbClient struct {
	client *authzed.Client
}

func InitClient() (*SpiceDbClient, error) {
	client, err := authzed.NewClient(
		SpicedbAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpcutil.WithInsecureBearerToken(GrpcPresharedKey),
	)

	if err != nil {
		return nil, err
	}

	return &SpiceDbClient{
		client: client,
	}, nil
}

func (c *SpiceDbClient) AddRelationships(ctx context.Context, rr []RelReq) error {
	var updates []*v1.RelationshipUpdate

	// mb neeed batch
	for _, v := range rr {
		updates = append(updates, &v1.RelationshipUpdate{
			Operation: v1.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &v1.Relationship{
				Resource: &v1.ObjectReference{
					ObjectType: v.ObjectType,
					ObjectId:   v.ObjectID,
				},
				Relation: v.Relation,
				Subject: &v1.SubjectReference{
					Object: &v1.ObjectReference{
						ObjectType: v.SubjectType,
						ObjectId:   v.SubjectID,
					},
				},
			}},
		)
	}

	batches := make([][]*v1.RelationshipUpdate, 0, len(updates)/BatchSize+1)
	start := 0
	end := BatchSize

	for i := 0; i <= len(updates)/BatchSize; i++ {
		batches = append(batches, updates[start:end])
		start += BatchSize
		end += BatchSize
		if end > len(updates) {
			end = len(updates)
		}
	}

	for _, batch := range batches {
		req := &v1.WriteRelationshipsRequest{
			Updates: batch,
		}

		_, err := c.client.WriteRelationships(ctx, req)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *SpiceDbClient) WriteSchema(ctx context.Context, data []byte) error {
	request := &v1.WriteSchemaRequest{Schema: string(data)}

	_, err := c.client.WriteSchema(ctx, request)

	if err != nil {
		return err
	}

	return nil
}

func (c *SpiceDbClient) CheckPermission(ctx context.Context, cr *CheckReq) (bool, error) {
	req := &v1.CheckPermissionRequest{
		Consistency: &v1.Consistency{
			Requirement: &v1.Consistency_MinimizeLatency{
				MinimizeLatency: true, // TODO
			},
		},
		Resource: &v1.ObjectReference{
			ObjectType: cr.ObjectType,
			ObjectId:   cr.ObjectID,
		},
		Permission: cr.Permission,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: cr.SubjectType,
				ObjectId:   cr.SubjectID,
			},
		},
	}

	resp, err := c.client.CheckPermission(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION, nil
}
