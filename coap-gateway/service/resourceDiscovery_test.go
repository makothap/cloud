package service_test

import (
	"context"
	"testing"

	testCfg "github.com/plgd-dev/cloud/test/config"
	"github.com/plgd-dev/go-coap/v2/tcp"

	coapCodes "github.com/plgd-dev/go-coap/v2/message/codes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resourceDirectoryFind(t *testing.T) {
	shutdown := setUp(t)
	defer shutdown()

	co := testCoapDial(t, testCfg.GW_HOST)
	if co == nil {
		return
	}
	defer co.Close()

	type args struct {
		queries []string
	}
	tests := []struct {
		name      string
		args      args
		wantsCode coapCodes.Code
	}{
		{
			name:      "without query",
			args:      args{},
			wantsCode: coapCodes.Content,
		},
		{
			name: "with di",
			args: args{
				queries: []string{"di=" + CertIdentity},
			},
			wantsCode: coapCodes.Content,
		},
		{
			name: "with rt",
			args: args{
				queries: []string{"rt=" + TestBResourceType},
			},
			wantsCode: coapCodes.Content,
		},
		{
			name: "with di & rt",
			args: args{
				queries: []string{"di=" + CertIdentity, "rt=" + TestAResourceType},
			},
			wantsCode: coapCodes.Content,
		},
		{
			name: "di not exist",
			args: args{
				queries: []string{"di=1234"},
			},
			wantsCode: coapCodes.NotFound,
		},
	}

	testPrepareDevice(t, co)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), TestExchangeTimeout)
			defer cancel()
			req, err := tcp.NewGetRequest(ctx, "/oic/res")
			require.NoError(t, err)
			for _, q := range tt.args.queries {
				req.AddQuery(q)
			}
			resp, err := co.Do(req)
			require.NoError(t, err)
			assert.Equal(t, tt.wantsCode, resp.Code())
		})
	}
}
