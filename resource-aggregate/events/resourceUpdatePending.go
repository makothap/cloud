package events

import (
	"google.golang.org/protobuf/proto"
)

const eventTypeResourceUpdatePending = "ocf.cloud.resourceaggregate.events.resourceupdatepending"

func (e *ResourceUpdatePending) Version() uint64 {
	return e.GetEventMetadata().GetVersion()
}

func (e *ResourceUpdatePending) Marshal() ([]byte, error) {
	return proto.Marshal(e)
}

func (e *ResourceUpdatePending) Unmarshal(b []byte) error {
	return proto.Unmarshal(b, e)
}

func (e *ResourceUpdatePending) EventType() string {
	return eventTypeResourceUpdatePending
}

func (e *ResourceUpdatePending) AggregateId() string {
	return e.GetResourceId().ToUUID()
}
