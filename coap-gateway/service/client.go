package service

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/plgd-dev/cloud/coap-gateway/coapconv"
	"github.com/plgd-dev/cloud/coap-gateway/schema/device/status"
	grpcClient "github.com/plgd-dev/cloud/grpc-gateway/client"
	"github.com/plgd-dev/cloud/grpc-gateway/pb"
	pbGRPC "github.com/plgd-dev/cloud/grpc-gateway/pb"
	kitNetGrpc "github.com/plgd-dev/cloud/pkg/net/grpc"
	"github.com/plgd-dev/cloud/resource-aggregate/commands"
	"github.com/plgd-dev/go-coap/v2/message"
	"github.com/plgd-dev/go-coap/v2/message/codes"
	coapCodes "github.com/plgd-dev/go-coap/v2/message/codes"
	"github.com/plgd-dev/go-coap/v2/tcp"
	"github.com/plgd-dev/go-coap/v2/tcp/message/pool"
	"github.com/plgd-dev/kit/log"
	kitSync "github.com/plgd-dev/kit/sync"
	"github.com/plgd-dev/sdk/schema"
)

const OCFBaselineInterface = "oic.if.baseline"

type observedResource struct {
	href string

	mutex       sync.Mutex
	observation *tcp.Observation
}

func (r *observedResource) SetObservation(o *tcp.Observation) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.observation = o
}

func (r *observedResource) PopObservation() *tcp.Observation {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	o := r.observation
	r.observation = nil
	return o
}

type authorizationContext struct {
	DeviceID    string
	AccessToken string
	UserID      string
	Expire      time.Time
}

func (a *authorizationContext) GetUserID() string {
	if a == nil {
		return ""
	}
	return a.UserID
}

func (a *authorizationContext) GetDeviceID() string {
	if a != nil {
		return a.DeviceID
	}
	return ""
}

func (a *authorizationContext) IsValid() error {
	if a == nil {
		return fmt.Errorf("invalid authorization context")
	}
	if a.AccessToken == "" {
		return fmt.Errorf("invalid access token")
	}
	if !a.Expire.IsZero() && time.Now().After(a.Expire) {
		return fmt.Errorf("token is expired")
	}
	return nil
}

const pendingDeviceSubscriptionToken = "pending"

//Client a setup of connection
type Client struct {
	server   *Server
	coapConn *tcp.ClientConn

	observedResources     map[string]map[int64]*observedResource // [deviceID][instanceID]
	observedResourcesLock sync.Mutex

	resourceSubscriptions *kitSync.Map // [token]

	mutex                    sync.Mutex
	authCtx                  *authorizationContext
	cancelDeviceSubscription func(ctx context.Context) error
}

//newClient create and initialize client
func newClient(server *Server, client *tcp.ClientConn) *Client {
	return &Client{
		server:                server,
		coapConn:              client,
		observedResources:     make(map[string]map[int64]*observedResource),
		resourceSubscriptions: kitSync.NewMap(),
	}
}

func ToClient(v interface{}, ok bool) (*Client, bool) {
	if !ok {
		return nil, false
	}
	if v == nil {
		return nil, false
	}
	c, ok := v.(*Client)
	return c, ok
}

func (client *Client) remoteAddrString() string {
	return client.coapConn.RemoteAddr().String()
}

func (client *Client) Context() context.Context {
	return client.coapConn.Context()
}

func (client *Client) cancelResourceSubscription(token string, wantWait bool) (bool, error) {
	s, ok := grpcClient.ToResourceSubscription(client.resourceSubscriptions.PullOut(token))
	if !ok {
		return false, nil
	}
	wait, err := s.Cancel()
	if err != nil {
		return false, err
	}
	if wantWait {
		wait()
	}
	return true, nil
}

func (client *Client) observeResource(ctx context.Context, resourceID *commands.ResourceId, observable, allowDuplicit bool) (err error) {
	log.Debugf("observation of resource %v requested", resourceID)
	instanceID := getInstanceID(resourceID.GetHref())
	client.observedResourcesLock.Lock()
	defer client.observedResourcesLock.Unlock()
	if _, ok := client.observedResources[resourceID.GetDeviceId()]; !ok {
		client.observedResources[resourceID.GetDeviceId()] = make(map[int64]*observedResource)
	}
	if _, ok := client.observedResources[resourceID.GetDeviceId()][instanceID]; ok {
		if allowDuplicit {
			return nil
		}
		return fmt.Errorf("resource is already already being observed")
	}
	obsRes := observedResource{href: resourceID.GetHref()}
	client.observedResources[resourceID.GetDeviceId()][instanceID] = &obsRes
	return client.server.taskQueue.Submit(func() { client.addObservedResourceLocked(ctx, resourceID.GetDeviceId(), observable, &obsRes) })
}

func (client *Client) getResourceContent(ctx context.Context, deviceID, href string) {
	resp, err := client.coapConn.Get(ctx, href, message.Option{
		ID:    message.URIQuery,
		Value: []byte("if=" + OCFBaselineInterface),
	})
	if err != nil {
		log.Errorf("cannot get resource /%v%v content: %v", deviceID, href, err)
		return
	}
	client.server.taskQueue.Submit(func() {
		defer pool.ReleaseMessage(resp)
		err = client.notifyContentChanged(deviceID, href, resp)
		if err != nil {
			// cloud is unsynchronized against device. To recover cloud state, client need to reconnect to cloud.
			log.Errorf("cannot get resource /%v%v content: %v", deviceID, href, err)
			client.Close()
		}
		if resp.Code() == coapCodes.NotFound {
			client.unpublishResourceLinks(client.getUserAuthorizedContext(ctx), []string{href})
		}
	})
}

func (client *Client) addObservedResourceLocked(ctx context.Context, deviceID string, isObservable bool, obsRes *observedResource) {
	if obsRes.href == commands.StatusHref {
		return
	}
	if isObservable {
		obs, err := client.coapConn.Observe(ctx, obsRes.href, func(req *pool.Message) {
			req.Hijack()
			client.server.taskQueue.Submit(func() {
				err := client.notifyContentChanged(deviceID, obsRes.href, req)
				defer pool.ReleaseMessage(req)
				if err != nil {
					// cloud is unsynchronized against device. To recover cloud state, client need to reconnect to cloud.
					log.Errorf("cannot observe resource /%v%v: %v", deviceID, obsRes.href, err)
					client.Close()
				}
				if req.Code() == coapCodes.NotFound {
					client.unpublishResourceLinks(client.getUserAuthorizedContext(req.Context()), []string{obsRes.href})
				}
			})
		}, message.Option{
			ID:    message.URIQuery,
			Value: []byte("if=" + OCFBaselineInterface),
		})
		if err != nil {
			log.Errorf("cannot observe resource /%v%v: %v", deviceID, obsRes.href, err)
		} else {
			obsRes.SetObservation(obs)
		}
	} else {
		client.getResourceContent(ctx, deviceID, obsRes.href)
	}
}

func (client *Client) getTrackedResources(deviceID string, instanceIDs []int64) []commands.ResourceId {
	client.observedResourcesLock.Lock()
	defer client.observedResourcesLock.Unlock()

	getAllDeviceIDMatches := len(instanceIDs) == 0
	matches := make([]commands.ResourceId, 0, 16)

	if deviceResourcesMap, ok := client.observedResources[deviceID]; ok {
		if getAllDeviceIDMatches {
			for _, value := range deviceResourcesMap {
				matches = append(matches, commands.ResourceId{
					DeviceId: deviceID,
					Href:     value.href,
				})
			}
		} else {
			for _, instanceID := range instanceIDs {
				if resource, ok := deviceResourcesMap[instanceID]; ok {
					matches = append(matches, commands.ResourceId{
						DeviceId: deviceID,
						Href:     resource.href,
					})
				}
			}
		}
	}

	return matches
}

func (client *Client) removeResource(deviceID string, instanceID int64) {
	if device, ok := client.observedResources[deviceID]; ok {
		delete(device, instanceID)
		if len(client.observedResources[deviceID]) == 0 {
			delete(client.observedResources, deviceID)
		}
	}
}

func (client *Client) popObservation(deviceID string, instanceID int64) *tcp.Observation {
	log.Debugf("remove published resource ocf://%v/%v", deviceID, instanceID)

	if device, ok := client.observedResources[deviceID]; ok {
		if res, ok := device[instanceID]; ok {
			return res.PopObservation()
		}
	}

	return nil
}

func (client *Client) cancelResourcesObservations(ctx context.Context, hrefs []string) {
	observartions := client.popTrackedObservation(hrefs)
	for _, obs := range observartions {
		obs.Cancel(ctx)
	}
}

// Close closes coap connection
func (client *Client) Close() error {
	err := client.coapConn.Close()
	if err != nil {
		return fmt.Errorf("cannot close client: %w", err)
	}
	return nil
}

func (client *Client) popTrackedObservation(hrefs []string) []*tcp.Observation {
	observartions := make([]*tcp.Observation, 0, 32)

	client.observedResourcesLock.Lock()
	defer client.observedResourcesLock.Unlock()

	for _, href := range hrefs {
		var instanceID int64
		var deviceID string
		for devID, devs := range client.observedResources {
			for insID, r := range devs {
				if r.href == href {
					instanceID = insID
					deviceID = devID
					break
				}
			}
		}

		obs := client.popObservation(deviceID, instanceID)
		if obs != nil {
			observartions = append(observartions, obs)
		}
		client.removeResource(deviceID, instanceID)
		log.Debugf("canceling observation on resource %v", deviceID+href)
	}
	return observartions
}

func (client *Client) popObservedResources() []*tcp.Observation {
	observartions := make([]*tcp.Observation, 0, 32)
	client.observedResourcesLock.Lock()
	defer client.observedResourcesLock.Unlock()
	for deviceID, instanceIDs := range client.observedResources {
		for instanceID := range instanceIDs {
			obs := client.popObservation(deviceID, instanceID)
			if obs != nil {
				observartions = append(observartions, obs)
			}
			client.removeResource(deviceID, instanceID)
		}
	}
	return observartions
}

// cleanObservedResources remove all device pbRA observation requested by cloud.
func (client *Client) cleanObservedResources() {
	for _, obs := range client.popObservedResources() {
		obs.Cancel(client.coapConn.Context())
	}
}

func (client *Client) cancelResourceSubscriptions(wantWait bool) {
	resourceSubscriptions := client.resourceSubscriptions.PullOutAll()
	for _, v := range resourceSubscriptions {
		o, ok := grpcClient.ToResourceSubscription(v, true)
		if !ok {
			continue
		}
		wait, err := o.Cancel()
		if err != nil {
			log.Errorf("cannot cancel resource subscription: %v", err)
		} else if wantWait {
			wait()
		}
	}
}

func (client *Client) unsetCancelDeviceSubscription() func(ctx context.Context) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	c := client.cancelDeviceSubscription
	client.cancelDeviceSubscription = nil
	return c
}

func (client *Client) storeDeviceSubscription(cancel func(ctx context.Context) error) bool {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if client.cancelDeviceSubscription != nil {
		return false
	}
	client.cancelDeviceSubscription = cancel
	return true
}

func (client *Client) cancelDeviceSubscriptions(ctx context.Context) {
	cancel := client.unsetCancelDeviceSubscription()
	if cancel != nil {
		cancel(ctx)
	}
}

func (client *Client) CleanUp() *authorizationContext {
	authCtx, _ := client.GetAuthorizationContext()
	log.Debugf("cleanUp client %v for device %v", client.coapConn.RemoteAddr(), authCtx.GetDeviceID())

	client.server.devicesStatusUpdater.Remove(client)
	client.server.oicPingCache.Delete(client.remoteAddrString())
	client.cleanObservedResources()
	client.cancelResourceSubscriptions(false)

	ctx, cancel := context.WithTimeout(client.server.ctx, client.server.RequestTimeout)
	defer cancel()
	client.cancelDeviceSubscriptions(ctx)

	return client.SetAuthorizationContext(nil)
}

// OnClose action when coap connection was closed.
func (client *Client) OnClose() {
	authCtx, _ := client.GetAuthorizationContext()
	log.Debugf("close client %v for device %v", client.coapConn.RemoteAddr(), authCtx.GetDeviceID())
	oldAuthCtx := client.CleanUp()

	if oldAuthCtx.GetDeviceID() != "" {
		client.server.expirationClientCache.Set(oldAuthCtx.GetDeviceID(), nil, time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), client.server.RequestTimeout)
		defer cancel()
		token, err := client.server.oauthMgr.GetToken(ctx)
		if err != nil {
			log.Errorf("DeviceId %v: cannot handle sign out: cannot update cloud device status: %v", oldAuthCtx.GetDeviceID(), err)
			return
		}
		err = status.SetOffline(kitNetGrpc.CtxWithOwner(kitNetGrpc.CtxWithToken(ctx, token.AccessToken), oldAuthCtx.GetUserID()), client.server.raClient, oldAuthCtx.GetDeviceID(), &commands.CommandMetadata{
			Sequence:     client.coapConn.Sequence(),
			ConnectionId: client.remoteAddrString(),
		})
		if err != nil {
			// Device will be still reported as online and it can fix his state by next calls online, offline commands.
			log.Errorf("DeviceId %v: cannot handle sign out: cannot update cloud device status: %v", oldAuthCtx.GetDeviceID(), err)
		}
	}
}

func (client *Client) SetAuthorizationContext(authCtx *authorizationContext) (oldDeviceID *authorizationContext) {
	log.Debugf("Authorization context replaced for client %v, device %v, user %v", client.coapConn.RemoteAddr(), authCtx.GetDeviceID(), authCtx.GetUserID())
	client.mutex.Lock()
	defer client.mutex.Unlock()
	oldAuthContext := client.authCtx
	client.authCtx = authCtx
	return oldAuthContext
}

func (client *Client) GetAuthorizationContext() (*authorizationContext, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	return client.authCtx, client.authCtx.IsValid()
}

func (client *Client) notifyContentChanged(deviceID string, href string, notification *pool.Message) error {
	authCtx, err := client.GetAuthorizationContext()
	if err != nil {
		return fmt.Errorf("cannot notify resource /%v%v content changed: %w", deviceID, href, err)
	}
	decodeMsgToDebug(client, notification, "RECEIVED-NOTIFICATION")
	ctx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
	if err != nil {
		return fmt.Errorf("cannot notify resource /%v%v content changed: %w", deviceID, href, err)
	}
	request := coapconv.NewNotifyResourceChangedRequest(commands.NewResourceID(deviceID, href), client.remoteAddrString(), notification)
	_, err = client.server.raClient.NotifyResourceChanged(ctx, request)
	if err != nil {
		return fmt.Errorf("cannot notify resource /%v%v content changed: %w", deviceID, href, err)
	}
	return nil
}

func (client *Client) sendErrorConfirmResourceUpdate(deviceID, href, userID, correlationID string, code codes.Code, errToSend error) {
	ctx, err := client.server.ServiceRequestContext(userID)
	if err != nil {
		log.Errorf("cannot send error via confirm resource update: %v", err)
		return
	}

	resp := pool.AcquireMessage(ctx)
	defer pool.ReleaseMessage(resp)
	resp.SetContentFormat(message.TextPlain)
	resp.SetBody(bytes.NewReader([]byte(errToSend.Error())))
	resp.SetCode(code)
	request := coapconv.NewConfirmResourceUpdateRequest(commands.NewResourceID(deviceID, href), correlationID, client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceUpdate(ctx, request)
	if err != nil {
		log.Errorf("cannot send error via confirm resource update: %v", err)
	}
}

func (client *Client) updateResource(ctx context.Context, event *pb.Event_ResourceUpdatePending) error {
	authCtx, err := client.GetAuthorizationContext()
	if err != nil {
		err := fmt.Errorf("cannot update resource /%v%v: %w", event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), err)
		client.sendErrorConfirmResourceUpdate(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.Forbidden, err)
		client.Close()
		return err
	}
	if event.GetResourceId().GetHref() == commands.StatusHref {
		msg := pool.AcquireMessage(ctx)
		msg.SetCode(coapCodes.MethodNotAllowed)
		msg.SetSequence(client.coapConn.Sequence())
		defer pool.ReleaseMessage(msg)
		sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
		if err != nil {
			return err
		}
		request := coapconv.NewConfirmResourceUpdateRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), msg)
		_, err = client.server.raClient.ConfirmResourceUpdate(sendConfirmCtx, request)
		if err != nil {
			return err
		}
		return nil
	}

	coapCtx, cancel := context.WithTimeout(ctx, client.server.RequestTimeout)
	defer cancel()
	req, err := coapconv.NewCoapResourceUpdateRequest(coapCtx, event)
	if err != nil {
		client.sendErrorConfirmResourceUpdate(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.BadRequest, err)
		return err
	}
	defer pool.ReleaseMessage(req)

	decodeMsgToDebug(client, req, "RESOURCE-UPDATE-REQUEST")

	resp, err := client.coapConn.Do(req)
	if err != nil {
		client.sendErrorConfirmResourceUpdate(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.ServiceUnavailable, err)
		return err
	}
	defer pool.ReleaseMessage(resp)

	decodeMsgToDebug(client, resp, "RESOURCE-UPDATE-RESPONSE")

	if resp.Code() == coapCodes.NotFound {
		client.unpublishResourceLinks(client.getUserAuthorizedContext(ctx), []string{event.GetResourceId().GetHref()})
	}

	sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
	if err != nil {
		return err
	}
	request := coapconv.NewConfirmResourceUpdateRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceUpdate(sendConfirmCtx, request)
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) sendErrorConfirmResourceRetrieve(deviceID, href, userID, correlationID string, code codes.Code, errToSend error) {
	ctx, err := client.server.ServiceRequestContext(userID)
	if err != nil {
		log.Errorf("cannot send error via confirm resource retrieve: %v", err)
		return
	}
	resp := pool.AcquireMessage(ctx)
	defer pool.ReleaseMessage(resp)
	resp.SetContentFormat(message.TextPlain)
	resp.SetBody(bytes.NewReader([]byte(errToSend.Error())))
	resp.SetCode(code)
	request := coapconv.NewConfirmResourceRetrieveRequest(commands.NewResourceID(deviceID, href), correlationID, client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceRetrieve(ctx, request)
	if err != nil {
		log.Errorf("cannot send error confirm resource retrieve: %v", err)
	}
}

func (client *Client) retrieveResource(ctx context.Context, event *pb.Event_ResourceRetrievePending) error {
	authCtx, err := client.GetAuthorizationContext()
	if err != nil {
		err := fmt.Errorf("cannot retrieve resource /%v%v: %w", event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), err)
		client.sendErrorConfirmResourceUpdate(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.Forbidden, err)
		client.Close()
		return err
	}

	if event.GetResourceId().GetHref() == commands.StatusHref {
		msg := pool.AcquireMessage(ctx)
		msg.SetCode(coapCodes.Content)
		msg.SetSequence(client.coapConn.Sequence())
		defer pool.ReleaseMessage(msg)

		sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
		if err != nil {
			return err
		}
		request := coapconv.NewConfirmResourceRetrieveRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), msg)
		_, err = client.server.raClient.ConfirmResourceRetrieve(sendConfirmCtx, request)
		if err != nil {
			return err
		}
		return nil
	}

	coapCtx, cancel := context.WithTimeout(ctx, client.server.RequestTimeout)
	defer cancel()
	req, err := coapconv.NewCoapResourceRetrieveRequest(coapCtx, event)
	if err != nil {
		client.sendErrorConfirmResourceUpdate(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.BadRequest, err)
		return err
	}
	defer pool.ReleaseMessage(req)

	decodeMsgToDebug(client, req, "RESOURCE-RETRIEVE-REQUEST")

	resp, err := client.coapConn.Do(req)
	if err != nil {
		client.sendErrorConfirmResourceUpdate(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.ServiceUnavailable, err)
		return err
	}
	defer pool.ReleaseMessage(resp)

	decodeMsgToDebug(client, resp, "RESOURCE-RETRIEVE-RESPONSE")

	if resp.Code() == coapCodes.NotFound {
		client.unpublishResourceLinks(client.getUserAuthorizedContext(ctx), []string{event.GetResourceId().GetHref()})
	}

	sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
	if err != nil {
		return err
	}
	request := coapconv.NewConfirmResourceRetrieveRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceRetrieve(sendConfirmCtx, request)
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) sendErrorConfirmResourceDelete(deviceID, href, userID, correlationID string, code codes.Code, errToSend error) {
	ctx, err := client.server.ServiceRequestContext(userID)
	if err != nil {
		log.Errorf("cannot send error via confirm resource delete: %v", err)
		return
	}

	resp := pool.AcquireMessage(ctx)
	defer pool.ReleaseMessage(resp)
	resp.SetContentFormat(message.TextPlain)
	resp.SetBody(bytes.NewReader([]byte(errToSend.Error())))
	resp.SetCode(code)
	request := coapconv.NewConfirmResourceDeleteRequest(commands.NewResourceID(deviceID, href), correlationID, client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceDelete(ctx, request)
	if err != nil {
		log.Errorf("cannot send error via confirm resource delete: %v", err)
	}
}

func (client *Client) deleteResource(ctx context.Context, event *pb.Event_ResourceDeletePending) error {
	authCtx, err := client.GetAuthorizationContext()
	if err != nil {
		err := fmt.Errorf("cannot delete resource /%v%v: %w", event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), err)
		client.sendErrorConfirmResourceDelete(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.Forbidden, err)
		client.Close()
		return err
	}

	if event.GetResourceId().GetHref() == commands.StatusHref {
		msg := pool.AcquireMessage(ctx)
		msg.SetCode(coapCodes.Forbidden)
		msg.SetSequence(client.coapConn.Sequence())
		defer pool.ReleaseMessage(msg)

		sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
		if err != nil {
			return err
		}
		request := coapconv.NewConfirmResourceDeleteRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), msg)
		_, err = client.server.raClient.ConfirmResourceDelete(sendConfirmCtx, request)
		if err != nil {
			return err
		}
		return nil
	}

	coapCtx, cancel := context.WithTimeout(ctx, client.server.RequestTimeout)
	defer cancel()
	req, err := coapconv.NewCoapResourceDeleteRequest(coapCtx, event)
	if err != nil {
		client.sendErrorConfirmResourceDelete(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.BadRequest, err)
		return err
	}
	defer pool.ReleaseMessage(req)

	decodeMsgToDebug(client, req, "RESOURCE-DELETE-REQUEST")

	resp, err := client.coapConn.Do(req)
	if err != nil {
		client.sendErrorConfirmResourceDelete(event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), authCtx.GetUserID(), event.GetCorrelationId(), codes.ServiceUnavailable, err)
		return err
	}
	defer pool.ReleaseMessage(resp)

	decodeMsgToDebug(client, resp, "RESOURCE-DELETE-RESPONSE")

	if resp.Code() == coapCodes.NotFound {
		client.unpublishResourceLinks(client.getUserAuthorizedContext(ctx), []string{event.GetResourceId().GetHref()})
	}

	sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
	if err != nil {
		return err
	}
	request := coapconv.NewConfirmResourceDeleteRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceDelete(sendConfirmCtx, request)
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) publishResourceLinks(ctx context.Context, links schema.ResourceLinks, deviceID string, ttl int32, connectionID string, sequence uint64) ([]*commands.Resource, error) {
	resources := pbGRPC.SchemaResourceLinksToRAResources(links, ttl)
	request := commands.PublishResourceLinksRequest{
		Resources: resources,
		DeviceId:  deviceID,
		CommandMetadata: &commands.CommandMetadata{
			Sequence:     sequence,
			ConnectionId: connectionID,
		},
	}

	resp, err := client.server.raClient.PublishResourceLinks(ctx, &request)
	if err != nil {
		return nil, fmt.Errorf("error occured during resource links publish %w", err)
	}

	return resp.GetPublishedResources(), nil
}

func (client *Client) getUserAuthorizedContext(ctx context.Context) context.Context {
	authCtx, err := client.GetAuthorizationContext()
	if err != nil {
		log.Errorf("unable to load authorization context", err)
		return nil
	}

	token, err := client.server.oauthMgr.GetToken(ctx)
	if err != nil {
		log.Errorf("unable to get token", err)
		return nil
	}
	return kitNetGrpc.CtxWithOwner(kitNetGrpc.CtxWithToken(ctx, token.AccessToken), authCtx.GetUserID())
}

func (client *Client) unpublishResourceLinks(ctx context.Context, hrefs []string) {
	authCtx, err := client.GetAuthorizationContext()
	if err != nil {
		log.Errorf("unable to load authorization context during resource links publish for device", err)
	}

	resp, err := client.server.raClient.UnpublishResourceLinks(ctx, &commands.UnpublishResourceLinksRequest{
		Hrefs:    hrefs,
		DeviceId: authCtx.GetDeviceID(),
		CommandMetadata: &commands.CommandMetadata{
			ConnectionId: client.remoteAddrString(),
			Sequence:     client.coapConn.Sequence(),
		},
	})
	if err != nil {
		// unpublish resource is not critical -> resource can be still accessible
		// next resource update will return 'not found' what triggers a publish again
		fmt.Errorf("error occured during resource links unpublish for device %v %w", authCtx.GetDeviceID(), err)
	}

	client.cancelResourcesObservations(ctx, resp.UnpublishedHrefs)
}

func (client *Client) sendErrorConfirmResourceCreate(resourceID *commands.ResourceId, userID, correlationID string, code codes.Code, errToSend error) {
	ctx, err := client.server.ServiceRequestContext(userID)
	if err != nil {
		log.Errorf("cannot send error via confirm resource create: %v", err)
		return
	}

	resp := pool.AcquireMessage(ctx)
	defer pool.ReleaseMessage(resp)
	resp.SetContentFormat(message.TextPlain)
	resp.SetBody(bytes.NewReader([]byte(errToSend.Error())))
	resp.SetCode(code)
	request := coapconv.NewConfirmResourceCreateRequest(resourceID, correlationID, client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceCreate(ctx, request)
	if err != nil {
		log.Errorf("cannot send error via confirm resource create: %v", err)
	}
}

func (client *Client) createResource(ctx context.Context, event *pb.Event_ResourceCreatePending) error {
	authCtx, err := client.GetAuthorizationContext()
	if err != nil {
		err := fmt.Errorf("cannot create resource /%v%v: %w", event.GetResourceId().GetDeviceId(), event.GetResourceId().GetHref(), err)
		client.sendErrorConfirmResourceCreate(event.GetResourceId(), authCtx.GetUserID(), event.GetCorrelationId(), codes.Forbidden, err)
		client.Close()
		return err
	}

	if event.GetResourceId().GetHref() == commands.StatusHref {
		msg := pool.AcquireMessage(ctx)
		msg.SetCode(coapCodes.Forbidden)
		msg.SetSequence(client.coapConn.Sequence())
		defer pool.ReleaseMessage(msg)

		sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
		if err != nil {
			return err
		}
		request := coapconv.NewConfirmResourceCreateRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), msg)
		_, err = client.server.raClient.ConfirmResourceCreate(sendConfirmCtx, request)
		if err != nil {
			return err
		}
		return nil
	}

	coapCtx, cancel := context.WithTimeout(ctx, client.server.RequestTimeout)
	defer cancel()
	req, err := coapconv.NewCoapResourceCreateRequest(coapCtx, event)
	if err != nil {
		client.sendErrorConfirmResourceCreate(event.GetResourceId(), authCtx.GetUserID(), event.GetCorrelationId(), codes.BadRequest, err)
		return err
	}
	defer pool.ReleaseMessage(req)

	decodeMsgToDebug(client, req, "RESOURCE-CREATE-REQUEST")

	resp, err := client.coapConn.Do(req)
	if err != nil {
		client.sendErrorConfirmResourceCreate(event.GetResourceId(), authCtx.GetUserID(), event.GetCorrelationId(), codes.ServiceUnavailable, err)
		return err
	}
	defer pool.ReleaseMessage(resp)

	decodeMsgToDebug(client, resp, "RESOURCE-CREATE-RESPONSE")

	if resp.Code() == coapCodes.NotFound {
		client.unpublishResourceLinks(client.getUserAuthorizedContext(ctx), []string{event.GetResourceId().GetHref()})
	}

	sendConfirmCtx, err := client.server.ServiceRequestContext(authCtx.GetUserID())
	if err != nil {
		return err
	}
	request := coapconv.NewConfirmResourceCreateRequest(event.GetResourceId(), event.GetCorrelationId(), client.remoteAddrString(), resp)
	_, err = client.server.raClient.ConfirmResourceCreate(sendConfirmCtx, request)
	if err != nil {
		return err
	}

	return nil
}
