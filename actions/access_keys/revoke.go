package access_keys

import (
	"net/http"

	"github.com/BuxOrg/bux"
	"github.com/julienschmidt/httprouter"
	apirouter "github.com/mrz1836/go-api-router"
)

// create will make a new model using the services defined in the action object
func (a *Action) revoke(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {

	reqXPub, _ := bux.GetXpubFromRequest(req)

	// Parse the params
	params := apirouter.GetParams(req)
	key := params.GetString("key")

	// Create a new accessKey
	accessKey, err := a.Services.Bux.RevokeAccessKey(
		req.Context(),
		reqXPub,
		key,
	)
	if err != nil {
		apirouter.ReturnResponse(w, req, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Return response
	apirouter.ReturnResponse(w, req, http.StatusCreated, bux.DisplayModels(accessKey))
}
