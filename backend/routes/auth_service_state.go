package routes

import "goklippy/backend/logic"

var authService *logic.AuthService

func InitAuthService(service *logic.AuthService) {
	authService = service
}

func getAuthService() *logic.AuthService {
	if authService == nil {
		panic("routes auth service is not initialized")
	}
	return authService
}
