package test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/plgd-dev/cloud/resource-aggregate/cqrs/eventstore"
	"github.com/stretchr/testify/require"
)

type mockEvent struct {
	VersionI   uint64 `bson:"version"`
	EventTypeI string `bson:"eventtype"`
	Data       string
}

func (e mockEvent) Version() uint64 {
	return e.VersionI
}

func (e mockEvent) EventType() string {
	return e.EventTypeI
}

type mockEventHandler struct {
	lock   sync.Mutex
	events map[string]map[string][]eventstore.Event
}

func NewMockEventHandler() *mockEventHandler {
	return &mockEventHandler{events: make(map[string]map[string][]eventstore.Event)}
}

func (eh *mockEventHandler) SetElement(groupId, aggrageId string, e mockEvent) {
	var device map[string][]eventstore.Event
	var ok bool

	eh.lock.Lock()
	defer eh.lock.Unlock()
	if device, ok = eh.events[groupId]; !ok {
		device = make(map[string][]eventstore.Event)
		eh.events[groupId] = device
	}
	device[aggrageId] = append(device[aggrageId], e)
}

func (eh *mockEventHandler) Handle(ctx context.Context, iter eventstore.Iter) error {
	for {
		eu, ok := iter.Next(ctx)
		if !ok {
			break
		}
		if eu.EventType() == "" {
			return errors.New("cannot determine type of event")
		}
		var e mockEvent
		err := eu.Unmarshal(&e)
		if err != nil {
			return err
		}
		eh.SetElement(eu.GroupID(), eu.AggregateID(), e)
	}
	return nil
}

func (eh *mockEventHandler) SnapshotEventType() string { return "snapshot" }

// AcceptanceTest is the acceptance test that all implementations of EventStore
// should pass. It should manually be called from a test case in each
// implementation:
//
//   func TestEventStore(t *testing.T) {
//       ctx := context.Background() // Or other when testing namespaces.
//       store := NewEventStore()
//       eventstore.AcceptanceTest(t, ctx, store)
//   }
//
func AcceptanceTest(t *testing.T, ctx context.Context, store eventstore.EventStore) {
	AggregateID1 := "aggregateID1"
	AggregateID2 := "aggregateID2"
	AggregateID3 := "aggregateID3"
	type Path struct {
		GroupID     string
		AggregateID string
	}

	aggregateID1Path := Path{
		AggregateID: AggregateID1,
		GroupID:     "deviceId",
	}
	aggregateID2Path := Path{
		AggregateID: AggregateID2,
		GroupID:     "deviceId",
	}
	aggregateID3Path := Path{
		AggregateID: AggregateID3,
		GroupID:     "deviceId1",
	}

	eventsToSave := []eventstore.Event{
		mockEvent{
			EventTypeI: "test0",
		},
		mockEvent{
			VersionI:   1,
			EventTypeI: "test1",
		},
		mockEvent{
			VersionI:   2,
			EventTypeI: "test2",
		},
		mockEvent{
			VersionI:   3,
			EventTypeI: "test3",
		},
		mockEvent{
			VersionI:   4,
			EventTypeI: "test4",
		},
		mockEvent{
			VersionI:   5,
			EventTypeI: "test5",
		},
		mockEvent{
			VersionI:   4,
			EventTypeI: "aggr2-test6",
		},
		mockEvent{
			VersionI:   5,
			EventTypeI: "aggr2-test7",
		},
		mockEvent{
			VersionI:   6,
			EventTypeI: "aggr2-test8",
		},
	}

	t.Log("save no events")
	conExcep, err := store.Save(ctx, aggregateID1Path.GroupID, aggregateID1Path.AggregateID, nil)
	require.Error(t, err)
	require.False(t, conExcep)

	t.Log("save event, VersionI 0")
	conExcep, err = store.Save(ctx, aggregateID1Path.GroupID, aggregateID1Path.AggregateID, []eventstore.Event{
		eventsToSave[0],
	})
	require.NoError(t, err)
	require.False(t, conExcep)

	t.Log("save event, VersionI 1")
	conExcep, err = store.Save(ctx, aggregateID1Path.GroupID, aggregateID1Path.AggregateID, []eventstore.Event{
		eventsToSave[1],
	})
	require.NoError(t, err)
	require.False(t, conExcep)

	t.Log("try to save same event VersionI 1 twice")
	conExcep, err = store.Save(ctx, aggregateID1Path.GroupID, aggregateID1Path.AggregateID, []eventstore.Event{
		eventsToSave[1],
	})
	require.True(t, conExcep)
	require.NoError(t, err)

	t.Log("save event, VersionI 2")
	conExcep, err = store.Save(ctx, aggregateID1Path.GroupID, aggregateID1Path.AggregateID, []eventstore.Event{
		eventsToSave[2],
	})
	require.NoError(t, err)
	require.False(t, conExcep)

	t.Log("save multiple events, VersionI 3, 4 and 5")
	conExcep, err = store.Save(ctx, aggregateID1Path.GroupID, aggregateID1Path.AggregateID, []eventstore.Event{
		eventsToSave[3], eventsToSave[4], eventsToSave[5],
	})
	require.NoError(t, err)
	require.False(t, conExcep)

	t.Log("save event for another aggregate")
	conExcep, err = store.Save(ctx, aggregateID2Path.GroupID, aggregateID2Path.AggregateID, []eventstore.Event{
		eventsToSave[0]})
	require.NoError(t, err)
	require.False(t, conExcep)

	conExcep, err = store.Save(ctx, aggregateID2Path.GroupID, aggregateID2Path.AggregateID, []eventstore.Event{
		eventsToSave[6], eventsToSave[7], eventsToSave[8]})
	require.NoError(t, err)
	require.False(t, conExcep)

	t.Log("load events for non-existing aggregate")
	eh1 := NewMockEventHandler()
	err = store.LoadFromSnapshot(ctx, []eventstore.SnapshotQuery{{GroupID: "notExist"}}, eh1)
	require.NoError(t, err)
	require.Equal(t, 0, len(eh1.events))

	t.Log("load events")
	eh2 := NewMockEventHandler()
	err = store.LoadFromSnapshot(ctx, []eventstore.SnapshotQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
		},
	}, eh2)
	require.NoError(t, err)
	require.Equal(t, eventsToSave[:6], eh2.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])

	t.Log("load events from version")
	eh3 := NewMockEventHandler()
	err = store.LoadFromVersion(ctx, []eventstore.VersionQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
			Version:     eventsToSave[2].Version(),
		},
	}, eh3)
	require.NoError(t, err)
	require.Equal(t, eventsToSave[2:6], eh3.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])

	t.Log("load multiple aggregatess by all queries")
	eh4 := NewMockEventHandler()
	err = store.LoadFromVersion(ctx, []eventstore.VersionQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
		},
		{
			GroupID:     aggregateID2Path.GroupID,
			AggregateID: aggregateID2Path.AggregateID,
		},
	}, eh4)
	require.NoError(t, err)
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[1], eventsToSave[2], eventsToSave[3], eventsToSave[4], eventsToSave[5],
	}, eh4.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[6], eventsToSave[7], eventsToSave[8],
	}, eh4.events[aggregateID2Path.GroupID][aggregateID2Path.AggregateID])

	t.Log("load multiple aggregates by groupId")
	eh5 := NewMockEventHandler()
	err = store.LoadFromSnapshot(ctx, []eventstore.SnapshotQuery{
		{
			GroupID: aggregateID1Path.GroupID,
		},
	}, eh5)
	require.NoError(t, err)
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[1], eventsToSave[2], eventsToSave[3], eventsToSave[4], eventsToSave[5],
	}, eh5.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[6], eventsToSave[7], eventsToSave[8],
	}, eh5.events[aggregateID2Path.GroupID][aggregateID2Path.AggregateID])

	t.Log("load multiple aggregates by all")
	eh6 := NewMockEventHandler()
	conExcep, err = store.Save(ctx, aggregateID3Path.GroupID, aggregateID3Path.AggregateID, []eventstore.Event{eventsToSave[0]})
	require.NoError(t, err)
	require.False(t, conExcep)
	err = store.LoadFromSnapshot(ctx, []eventstore.SnapshotQuery{{GroupID: aggregateID1Path.GroupID}, {GroupID: aggregateID2Path.GroupID}, {GroupID: aggregateID3Path.GroupID}}, eh6)
	require.NoError(t, err)
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[1], eventsToSave[2], eventsToSave[3], eventsToSave[4], eventsToSave[5],
	}, eh6.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[6], eventsToSave[7], eventsToSave[8],
	}, eh6.events[aggregateID2Path.GroupID][aggregateID2Path.AggregateID])
	require.Equal(t, []eventstore.Event{
		eventsToSave[0],
	}, eh6.events[aggregateID3Path.GroupID][aggregateID3Path.AggregateID])

	t.Log("load events up to version")
	eh7 := NewMockEventHandler()
	err = store.LoadUpToVersion(ctx, []eventstore.VersionQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
			Version:     eventsToSave[5].Version(),
		},
	}, eh7)
	require.NoError(t, err)
	require.Equal(t, eventsToSave[0:5], eh7.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])

	t.Log("load events up to version")
	eh8 := NewMockEventHandler()
	err = store.LoadUpToVersion(ctx, []eventstore.VersionQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
			Version:     eventsToSave[0].Version(),
		},
	}, eh8)
	require.NoError(t, err)
	require.Equal(t, 0, len(eh8.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID]))

	t.Log("load events up to version without version specified")
	eh9 := NewMockEventHandler()
	err = store.LoadUpToVersion(ctx, []eventstore.VersionQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
		},
	}, eh9)
	require.NoError(t, err)
	require.Equal(t, 0, len(eh9.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID]))

	t.Log("test projection all")
	model := NewMockEventHandler()
	p := eventstore.NewProjection(store, func(context.Context, string, string) (eventstore.Model, error) { return model, nil }, nil)

	err = p.Project(ctx, []eventstore.SnapshotQuery{{GroupID: aggregateID1Path.GroupID}, {GroupID: aggregateID2Path.GroupID}, {GroupID: aggregateID3Path.GroupID}})
	require.NoError(t, err)
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[1], eventsToSave[2], eventsToSave[3], eventsToSave[4], eventsToSave[5],
	}, model.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[6], eventsToSave[7], eventsToSave[8],
	}, model.events[aggregateID2Path.GroupID][aggregateID2Path.AggregateID])
	require.Equal(t, []eventstore.Event{
		eventsToSave[0],
	}, model.events[aggregateID3Path.GroupID][aggregateID3Path.AggregateID])

	t.Log("test projection group")
	model1 := NewMockEventHandler()
	p = eventstore.NewProjection(store, func(context.Context, string, string) (eventstore.Model, error) { return model1, nil }, nil)

	err = p.Project(ctx, []eventstore.SnapshotQuery{eventstore.SnapshotQuery{GroupID: aggregateID1Path.GroupID}})
	require.NoError(t, err)
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[1], eventsToSave[2], eventsToSave[3], eventsToSave[4], eventsToSave[5],
	}, model1.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[6], eventsToSave[7], eventsToSave[8],
	}, model1.events[aggregateID2Path.GroupID][aggregateID2Path.AggregateID])

	t.Log("test projection aggregate")
	model2 := NewMockEventHandler()
	p = eventstore.NewProjection(store, func(context.Context, string, string) (eventstore.Model, error) { return model2, nil }, nil)

	err = p.Project(ctx, []eventstore.SnapshotQuery{
		eventstore.SnapshotQuery{
			GroupID:     aggregateID2Path.GroupID,
			AggregateID: aggregateID2Path.AggregateID,
		},
	})
	require.NoError(t, err)
	require.Equal(t, []eventstore.Event{
		eventsToSave[0], eventsToSave[6], eventsToSave[7], eventsToSave[8],
	}, model2.events[aggregateID2Path.GroupID][aggregateID2Path.AggregateID])

	t.Log("remove events up to version")
	versionToRemove := 3
	err = store.RemoveUpToVersion(ctx, []eventstore.VersionQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
			Version:     eventsToSave[versionToRemove].Version(),
		},
	})
	require.NoError(t, err)

	eh10 := NewMockEventHandler()
	err = store.LoadFromVersion(ctx, []eventstore.VersionQuery{
		{
			GroupID:     aggregateID1Path.GroupID,
			AggregateID: aggregateID1Path.AggregateID,
		},
	}, eh10)
	require.NoError(t, err)
	require.Equal(t, eventsToSave[versionToRemove:6], eh10.events[aggregateID1Path.GroupID][aggregateID1Path.AggregateID])
}
