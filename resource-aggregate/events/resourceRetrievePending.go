package events

import (
	"google.golang.org/protobuf/proto"
)

const eventTypeResourceRetrievePending = "ocf.cloud.resourceaggregate.events.resourceretrievepending"

func (e *ResourceRetrievePending) Version() uint64 {
	return e.GetEventMetadata().GetVersion()
}

func (e *ResourceRetrievePending) Marshal() ([]byte, error) {
	return proto.Marshal(e)
}

func (e *ResourceRetrievePending) Unmarshal(b []byte) error {
	return proto.Unmarshal(b, e)
}

func (e *ResourceRetrievePending) EventType() string {
	return eventTypeResourceRetrievePending
}

func (e *ResourceRetrievePending) AggregateId() string {
	return e.GetResourceId().ToUUID()
}
