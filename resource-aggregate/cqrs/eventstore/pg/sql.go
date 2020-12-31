package pg

import (
	"context"
	"fmt"
	"hash/crc64"
	"strconv"
	"strings"

	"github.com/plgd-dev/cqrs/eventstore/maintenance"

	cqrsUtils "github.com/plgd-dev/cloud/resource-aggregate/cqrs"
	"github.com/plgd-dev/cqrs/event"
	"github.com/plgd-dev/cqrs/eventstore"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

// EventStore implements an EventStore for pgsql.
type EventStore struct {
	config Config
	db     *pg.DB
}

//NewEventStore create a event store from configuration
func NewEventStore(config Config, goroutinePoolGo eventstore.GoroutinePoolGoFunc, opts ...Option) (*EventStore, error) {
	config.marshalerFunc = cqrsUtils.Marshal
	config.unmarshalerFunc = cqrsUtils.Unmarshal
	config.logDebug = func(fmt string, args ...interface{}) {}
	for _, o := range opts {
		config = o(config)
	}
	db := pg.Connect(&pg.Options{
		User:     config.User,
		Addr:     config.Addr,
		Database: config.Database,
		Password: config.Password,
	})

	return &EventStore{
		config: config,
		db:     db,
	}, nil
}

type modeler interface {
	ModelContext(c context.Context, model ...interface{}) *orm.Query
}

func (s *EventStore) saveEvents(ctx context.Context, groupID, aggregateID string, events []event.Event) (concurrencyException bool, err error) {
	tx, err := s.db.Begin()
	// Make sure to close transaction if something goes wrong.
	defer tx.Close()

	for _, event := range events {
		cc, err := s.saveEvent(ctx, tx, groupID, aggregateID, event)
		if err != nil {
			tx.Rollback()
			return false, err
		}
		if cc {
			tx.Rollback()
			return true, nil
		}
	}

	err = tx.CommitContext(ctx)
	if err != nil {
		return false, err
	}
	return false, nil
}

func aggregateID2Hash(aggregateID string) int64 {
	h := crc64.New(crc64.MakeTable(crc64.ISO))
	h.Write([]byte(aggregateID))
	return int64(h.Sum64())
}

func (s *EventStore) saveEvent(ctx context.Context, m modeler, groupID, aggregateID string, event event.Event) (concurrencyException bool, err error) {
	raw, err := s.config.marshalerFunc(event)
	if err != nil {
		return false, fmt.Errorf("cannot marshal event data: %w", err)
	}
	storeEvent := Event{
		EventType:      event.EventType(),
		Version:        event.Version() + 1,
		AggregateIDStr: aggregateID,
		AggregateID:    aggregateID2Hash(aggregateID),
		Data:           raw,
	}
	if event.Version() == uint64(0) {
		err := eventTable(m.ModelContext(ctx, (*Event)(nil)), groupID).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
		})
		if err != nil {
			return false, fmt.Errorf("cannot create table: %w", err)
		}
	}
	res, err := eventTable(m.ModelContext(ctx, &storeEvent), groupID).OnConflict("DO NOTHING").Insert()
	if err != nil {
		return false, err
	}

	if res.RowsAffected() > 0 {
		if event.Version() == 0 {
			s.saveSnaphost(ctx, m, groupID, aggregateID, event.Version())
		}
		return false, nil
	}
	return true, nil
}

// Save saves events to a path.
func (s *EventStore) Save(ctx context.Context, groupID, aggregateID string, events []event.Event) (concurrencyException bool, err error) {
	if len(events) == 0 {
		return false, fmt.Errorf("no events")
	}
	if len(events) == 1 {
		return s.saveEvent(ctx, s.db, groupID, aggregateID, events[0])
	}
	return s.saveEvents(ctx, groupID, aggregateID, events)
}

func (s *EventStore) saveSnaphost(ctx context.Context, m modeler, groupID, aggregateID string, version uint64) error {
	snapshot := Snapshot{
		Version:     version + 1,
		AggregateID: aggregateID2Hash(aggregateID),
	}
	if version == uint64(0) {
		err := snapshotTable(m.ModelContext(ctx, (*Snapshot)(nil)), groupID).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
		})
		if err != nil {
			return fmt.Errorf("cannot create snapshot table: %w", err)
		}
	}

	res, err := snapshotTable(m.ModelContext(ctx, &snapshot), groupID).OnConflict("(aggregate_id) DO UPDATE").
		Set("version = ?", version+1).
		Insert()
	if err != nil {
		return err
	}

	if res.RowsAffected() > 0 {
		return nil
	}
	return fmt.Errorf("wtf?")
}

// SaveSnapshot saves snapshots to a path.
func (s *EventStore) SaveSnapshot(ctx context.Context, groupID, aggregateID string, ev event.Event) (concurrencyException bool, err error) {
	concurrencyException, err = s.Save(ctx, groupID, aggregateID, []event.Event{ev})
	if err != nil || concurrencyException {
		return concurrencyException, err
	}
	s.saveSnaphost(ctx, s.db, groupID, aggregateID, ev.Version())

	return false, nil
}

type iterator struct {
	events          []Event
	dataUnmarshaler event.UnmarshalerFunc
	LogDebugfFunc   LogDebugFunc
	err             error
	idx             int
	groupID         string
}

func (i *iterator) Next(ctx context.Context, e *event.EventUnmarshaler) bool {
	if i.err != nil {
		return false
	}
	if i.idx >= len(i.events) {
		return false
	}
	event := i.events[i.idx]
	i.idx++

	i.LogDebugfFunc("iterator.next: %+v", event)

	e.Version = event.Version - 1
	e.AggregateID = event.AggregateIDStr
	e.EventType = event.EventType
	e.GroupID = i.groupID
	e.Unmarshal = func(v interface{}) error {
		return i.dataUnmarshaler(event.Data, v)
	}
	return true
}

func (i *iterator) Err() error {
	return i.err
}

func (s *EventStore) loadByVersion(ctx context.Context, queries []eventstore.VersionQuery, eventHandler event.Handler, opVersion string) error {
	// Receive all stored values in order
	var errors []error
	for _, q := range queries {
		err := func(query eventstore.VersionQuery) error {
			var events []Event
			err := eventTable(s.db.ModelContext(ctx, &events), query.GroupID).Where("event.aggregate_id = ?", aggregateID2Hash(query.AggregateID)).Where("event.version "+opVersion+" ?", query.Version+1).Select()
			if err != nil {
				return err
			}
			i := iterator{
				events:          events,
				dataUnmarshaler: s.config.unmarshalerFunc,
				LogDebugfFunc:   s.config.logDebug,
				groupID:         query.GroupID,
			}
			return eventHandler.Handle(ctx, &i)
		}(q)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("%+v", errors)
	}

	return nil
}

// LoadFromVersion loads aggragate events from a specific version.
func (s *EventStore) LoadFromVersion(ctx context.Context, queries []eventstore.VersionQuery, eventHandler event.Handler) error {
	return s.loadByVersion(ctx, queries, eventHandler, ">=")
}

// LoadUpToVersion loads aggragate events up to a specific version.
func (s *EventStore) LoadUpToVersion(ctx context.Context, queries []eventstore.VersionQuery, eventHandler event.Handler) error {
	return s.loadByVersion(ctx, queries, eventHandler, "<")
}

func (s *EventStore) loadFromSnapshot(ctx context.Context, groupID string, queries []eventstore.SnapshotQuery, eventHandler event.Handler) error {
	withWhere := ""
	for _, q := range queries {
		if q.AggregateID != "" {
			if withWhere == "" {
				withWhere = "WHERE aggregate_id = " + strconv.FormatInt(int64(aggregateID2Hash(q.AggregateID)), 10)
			} else {
				withWhere += " OR aggregate_id = " + strconv.FormatInt(int64(aggregateID2Hash(q.AggregateID)), 10)
			}
		}
	}
	var events []Event

	res, err := s.db.Query(&events, `
	WITH tmp as (SELECT aggregate_id,version FROM `+snapshotTableName(groupID)+` `+withWhere+`)
	SELECT events.* from tmp, `+eventTableName(groupID)+` AS events
	WHERE events.aggregate_id = tmp.aggregate_id AND events.version >= tmp.version
	ORDER BY events.aggregate_id, events.version ASC;
	`)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil
		}
		return err
	}
	if res.RowsReturned() == 0 {
		return nil
	}

	i := iterator{
		events:          events,
		dataUnmarshaler: s.config.unmarshalerFunc,
		LogDebugfFunc:   s.config.logDebug,
		groupID:         groupID,
	}
	return eventHandler.Handle(ctx, &i)
}

// LoadFromSnapshot loads events from beginning.
func (s *EventStore) LoadFromSnapshot(ctx context.Context, queries []eventstore.SnapshotQuery, eventHandler event.Handler) error {
	if len(queries) == 0 {
		return fmt.Errorf("not supported")
	}
	collections := make(map[string][]eventstore.SnapshotQuery)
	for _, query := range queries {
		if query.GroupID == "" {
			continue
		}
		if query.AggregateID == "" {
			collections[query.GroupID] = make([]eventstore.SnapshotQuery, 0, 1)
			continue
		}
		v, ok := collections[query.GroupID]
		if !ok {
			v = make([]eventstore.SnapshotQuery, 0, 4)
		} else if len(v) == 0 {
			continue
		}
		v = append(v, query)
		collections[query.GroupID] = v
	}

	var errors []error
	for groupID, queries := range collections {
		err := s.loadFromSnapshot(ctx, groupID, queries, eventHandler)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%v", errors)
	}

	return nil
}

// RemoveUpToVersion deletes the aggragates events up to a specific version.
func (s *EventStore) RemoveUpToVersion(ctx context.Context, queries []eventstore.VersionQuery) error {
	// Receive all stored values in order
	var errors []error
	for _, q := range queries {
		err := func(query eventstore.VersionQuery) error {
			_, err := eventTable(s.db.ModelContext(ctx, (*Event)(nil)), query.GroupID).Where("event.aggregate_id = ?", aggregateID2Hash(query.AggregateID)).Where("event.version < ?", query.Version+1).Delete()
			return err
		}(q)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("%+v", errors)
	}

	return nil
}

// Insert stores (or updates) the information about the latest snapshot version per aggregate into the DB
func (s *EventStore) Insert(ctx context.Context, task maintenance.Task) error {
	return nil
}

// Query retrieves the latest snapshot version per aggregate for thw number of aggregates specified by 'limit'
func (s *EventStore) Query(ctx context.Context, limit int, taskHandler maintenance.TaskHandler) error {
	return fmt.Errorf("not supported")
}

// Remove deletes (the latest snapshot version) database record for a given aggregate ID
func (s *EventStore) Remove(ctx context.Context, task maintenance.Task) error {
	return fmt.Errorf("not supported")
}

// Clear clears the event storage.
func (s *EventStore) Clear(ctx context.Context) error {
	var tableInfo []struct {
		Table string
	}
	query := `SELECT table_name "table"
				FROM information_schema.tables WHERE table_schema='public' AND (table_name LIKE 'e_%' OR table_name LIKE 's_%');`
	_, err := s.db.Query(&tableInfo, query)
	if err != nil {
		return err
	}
	var errors []error
	for idx := range tableInfo {
		err = s.db.ModelContext(ctx, (*Event)(nil)).Table(tableInfo[idx].Table).DropTable(nil)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("%+v", errors)
	}
	return nil
}

// Close closes the database session.
func (s *EventStore) Close(ctx context.Context) error {
	return s.db.Close()
}
