package pb

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/plgd-dev/cloud/resource-aggregate/commands"
	"github.com/plgd-dev/cloud/resource-aggregate/events"
	"google.golang.org/grpc/peer"
)

func (req *UpdateResourceRequest) ToRACommand(ctx context.Context) (*commands.UpdateResourceRequest, error) {
	correlationUUID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	connectionID := ""
	peer, ok := peer.FromContext(ctx)
	if ok {
		connectionID = peer.Addr.String()
	}
	return &commands.UpdateResourceRequest{
		ResourceId:        req.GetResourceId(),
		CorrelationId:     correlationUUID.String(),
		ResourceInterface: req.GetResourceInterface(),
		Content: &commands.Content{
			Data:              req.GetContent().GetData(),
			ContentType:       req.GetContent().GetContentType(),
			CoapContentFormat: -1,
		},
		CommandMetadata: &commands.CommandMetadata{
			ConnectionId: connectionID,
		},
	}, nil
}

func RAResourceUpdatedEventToResponse(e *events.ResourceUpdated) (*UpdateResourceResponse, error) {
	content, err := EventContentToContent(e)
	if err != nil {
		return nil, err
	}
	return &UpdateResourceResponse{
		Content: content,
		Status:  RAStatus2Status(e.GetStatus()),
	}, nil
}
