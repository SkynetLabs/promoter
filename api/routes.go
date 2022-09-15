package api

type (
	// HealthGET is the type returned by the /health endpoint.
	HealthGET struct {
		DBAlive bool `json:"dbAlive"`
	}
)

// buildHTTPRoutes registers the http routes with the httprouter.
func (api *API) buildHTTPRoutes() {
	api.staticRouter.GET("/health", api.healthGET)
	api.staticRouter.POST("/payment", api.WithDBSession(api.paymentPOST))
}
