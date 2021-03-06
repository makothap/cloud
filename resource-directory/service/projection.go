package service

import (
	"context"
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/plgd-dev/cloud/resource-aggregate/commands"
	"github.com/plgd-dev/cloud/resource-aggregate/cqrs/eventbus"
	"github.com/plgd-dev/cloud/resource-aggregate/cqrs/eventstore"
	projectionRA "github.com/plgd-dev/cloud/resource-aggregate/cqrs/projection"
	"github.com/plgd-dev/cloud/resource-aggregate/events"
	"github.com/plgd-dev/kit/strings"
)

// hasMatchingType returns true for matching a resource type.
// An empty typeFilter matches all resource types.
func hasMatchingType(resourceTypes []string, typeFilter strings.Set) bool {
	if len(typeFilter) == 0 {
		return true
	}
	if len(resourceTypes) == 0 {
		return false
	}
	return typeFilter.HasOneOf(resourceTypes...)
}

type Projection struct {
	*projectionRA.Projection
	cache *cache.Cache
}

func NewProjection(ctx context.Context, name string, store eventstore.EventStore, subscriber eventbus.Subscriber, newModelFunc eventstore.FactoryModelFunc, expiration time.Duration) (*Projection, error) {
	projection, err := projectionRA.NewProjection(ctx, name, store, subscriber, newModelFunc)
	if err != nil {
		return nil, fmt.Errorf("cannot create server: %w", err)
	}
	cache := cache.New(expiration, expiration)
	cache.OnEvicted(func(deviceID string, _ interface{}) {
		projection.Unregister(deviceID)
	})
	return &Projection{Projection: projection, cache: cache}, nil
}

func (p *Projection) getModels(ctx context.Context, resourceID *commands.ResourceId) ([]eventstore.Model, error) {
	loaded, err := p.Register(ctx, resourceID.GetDeviceId())
	if err != nil {
		return nil, fmt.Errorf("cannot register to projection for %v: %w", resourceID, err)
	}
	if loaded {
		p.cache.Set(resourceID.GetDeviceId(), loaded, cache.DefaultExpiration)
	} else {
		defer func(ID string) {
			p.Unregister(ID)
		}(resourceID.GetDeviceId())
	}
	m := p.Models(resourceID)
	if !loaded && len(m) == 0 {
		err := p.ForceUpdate(ctx, resourceID)
		if err == nil {
			m = p.Models(resourceID)
		}
	}
	return m, nil
}

func (p *Projection) GetResourceLinks(ctx context.Context, deviceIDFilter, typeFilter strings.Set) (map[string]map[string]*commands.Resource, error) {
	devicesResourceLinks := make(map[string]map[string]*commands.Resource)
	for deviceID := range deviceIDFilter {
		models, err := p.getModels(ctx, commands.NewResourceID(deviceID, commands.ResourceLinksHref))
		if err != nil {
			return nil, err
		}
		if len(models) != 1 {
			return nil, nil
		}
		resourceLinks := models[0].(*resourceLinksProjection).Clone()
		devicesResourceLinks[resourceLinks.deviceID] = make(map[string]*commands.Resource)
		for href, resource := range resourceLinks.resources {
			if !hasMatchingType(resource.ResourceTypes, typeFilter) {
				continue
			}
			devicesResourceLinks[resourceLinks.deviceID][href] = resource
		}
	}

	return devicesResourceLinks, nil
}

func (p *Projection) GetResourcesWithLinks(ctx context.Context, resourceIDFilter []*commands.ResourceId, typeFilter strings.Set) (map[string]map[string]*Resource, error) {
	// group resource ID filter
	resourceIDMapFilter := make(map[string]map[string]bool)
	for _, resourceID := range resourceIDFilter {
		if resourceID.GetHref() == "" {
			resourceIDMapFilter[resourceID.GetDeviceId()] = nil
		} else {
			hrefs, present := resourceIDMapFilter[resourceID.GetDeviceId()]
			if present && hrefs == nil {
				continue
			}
			if !present {
				resourceIDMapFilter[resourceID.GetDeviceId()] = make(map[string]bool)
			}
			resourceIDMapFilter[resourceID.GetDeviceId()][resourceID.GetHref()] = true
		}
	}

	resources := make(map[string]map[string]*Resource)
	models := make([]eventstore.Model, 0, len(resourceIDMapFilter))
	for deviceID, hrefs := range resourceIDMapFilter {
		// build resource links map of all devices which are requested
		rl, err := p.GetResourceLinks(ctx, strings.Set{deviceID: {}}, nil)
		if err != nil {
			return nil, err
		}

		anyDeviceResourceFound := false
		resources[deviceID] = make(map[string]*Resource)
		if hrefs == nil {
			// case when client requests all device resources
			for _, resource := range rl[deviceID] {
				if hasMatchingType(resource.ResourceTypes, typeFilter) {
					resources[deviceID][resource.GetHref()] = &Resource{Resource: resource}
					anyDeviceResourceFound = true
				}
			}
		} else {
			// case when client requests specific device resource
			for href := range hrefs {
				if resource, present := rl[deviceID][href]; present {
					if hasMatchingType(resource.ResourceTypes, typeFilter) {
						resources[deviceID][href] = &Resource{Resource: resource}
						anyDeviceResourceFound = true
					}
				}
			}
		}

		if anyDeviceResourceFound {
			m, err := p.getModels(ctx, commands.NewResourceID(deviceID, ""))
			if err != nil {
				return nil, err
			}
			models = append(models, m...)
		} else {
			delete(resources, deviceID)
		}
	}

	for _, m := range models {
		if m.SnapshotEventType() == events.NewResourceLinksSnapshotTaken().SnapshotEventType() {
			continue
		}
		rp := m.(*resourceProjection).Clone()
		if _, present := resources[rp.resourceID.GetDeviceId()][rp.resourceID.GetHref()]; !present {
			continue
		}
		resources[rp.resourceID.GetDeviceId()][rp.resourceID.GetHref()].projection = rp
	}

	return resources, nil
}
