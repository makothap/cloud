package client_test

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	kitNetGrpc "github.com/plgd-dev/cloud/pkg/net/grpc"

	"github.com/plgd-dev/cloud/grpc-gateway/client"
	"github.com/plgd-dev/cloud/grpc-gateway/pb"
	test "github.com/plgd-dev/cloud/test"
	testCfg "github.com/plgd-dev/cloud/test/config"
	oauthTest "github.com/plgd-dev/cloud/test/oauth-server/test"
	"github.com/stretchr/testify/require"
)

const TestTimeout = time.Second * 20
const DeviceSimulatorIdNotFound = "00000000-0000-0000-0000-000000000111"

type testApplication struct {
	cas []*x509.Certificate
}

func (a *testApplication) GetRootCertificateAuthorities() ([]*x509.Certificate, error) {
	return a.cas, nil
}

func NewTestDeviceSimulator(deviceID, deviceName string, withResources bool) client.DeviceDetails {
	var resources []*pb.ResourceLink
	if withResources {
		resources = test.SortResources(test.ResourceLinksToPb(deviceID, test.GetAllBackendResourceLinks()))
	}

	return client.DeviceDetails{
		ID: deviceID,
		Device: &pb.Device{
			Id:         deviceID,
			Name:       deviceName,
			Types:      []string{"oic.d.cloudDevice", "oic.wk.d"},
			IsOnline:   true,
			Interfaces: []string{"oic.if.r", "oic.if.baseline"},
		},
		Resources: resources,
	}
}

func TestClient_GetDevice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()
	tearDown := test.SetUp(ctx, t)
	defer tearDown()

	deviceID := test.MustFindDeviceByName(test.TestDeviceName)
	type args struct {
		token    string
		deviceID string
	}
	tests := []struct {
		name    string
		args    args
		want    client.DeviceDetails
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				token:    oauthTest.GetServiceToken(t),
				deviceID: deviceID,
			},
			want: NewTestDeviceSimulator(deviceID, test.TestDeviceName, true),
		},
		{
			name: "not-found",
			args: args{
				token:    oauthTest.GetServiceToken(t),
				deviceID: "not-found",
			},
			wantErr: true,
		},
	}

	ctx = kitNetGrpc.CtxWithToken(ctx, oauthTest.GetServiceToken(t))

	c := NewTestClient(t)
	defer c.Close(context.Background())

	deviceID, shutdownDevSim := test.OnboardDevSim(ctx, t, c.GrpcGatewayClient(), deviceID, testCfg.GW_HOST, test.GetAllBackendResourceLinks())
	defer shutdownDevSim()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			got, err := c.GetDevice(ctx, tt.args.deviceID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got.Resources = test.SortResources(got.Resources)
			require.NotEmpty(t, got.Device.GetProtocolIndependentId())
			got.Device.ProtocolIndependentId = ""
			test.CheckProtobufs(t, tt.want, got, test.RequireToCheckFunc(require.Equal))
		})
	}
}
