package service

import (
	"github.com/go-ocf/go-coap/v2/mux"
)

func clientResetHandler(s mux.ResponseWriter, req *mux.Message, client *Client) {
	authCtx := client.loadAuthorizationContext()
	clientResetObservationHandler(s, req, client, authCtx.AuthorizationContext)
}