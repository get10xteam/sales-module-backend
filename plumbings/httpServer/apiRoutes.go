package httpServer

import (
	"github.com/get10xteam/sales-module-backend/app/client"
	"github.com/get10xteam/sales-module-backend/app/level"
	"github.com/get10xteam/sales-module-backend/app/opportunity"
	"github.com/get10xteam/sales-module-backend/app/status"
	"github.com/get10xteam/sales-module-backend/app/user"
	"github.com/get10xteam/sales-module-backend/plumbings/oauth"
	"github.com/get10xteam/sales-module-backend/plumbings/storage"

	"github.com/gofiber/fiber/v2"
)

func apiRoutes(apiRouter fiber.Router) {
	authRoute := apiRouter.Group("auth")
	{ // auth

		authRoute.Post("register", user.UserSignUpHandler)
		authRoute.Get("register", user.UserSignUpVerifyHandler)
		authRoute.Post("login", user.UserLoginHandler)
		authRoute.Get("profile", user.MustAuthMiddleware, user.UserProfileHandler)
		authRoute.Put("profile",
			user.MustAuthMiddleware,
			storage.UploadHandlerFactory(&storage.UploadConfig{
				NonFormCallNext:  true,
				CallNextWhenDone: true,
				AllowedTypes:     []string{"image"},
				MaxSize:          1 * 1024 * 1024,
				PathPrefix:       "usrProfileImg",
				PathIncludeHash:  true,
			}),
			user.ChangeProfileHandler,
		)
		authRoute.Post("pwreset", user.UserResetPasswordStartHandler)
		authRoute.Get("pwreset", user.UserResetPasswordSubmitHandler)
		authRoute.Put("pwreset", user.UserResetPasswordSubmitHandler)
		authRoute.Get("oauth/google", oauth.GoogleGetSignInUrlHandler)
		authRoute.Post("oauth/google", user.UserOauthLoginGoogleHandler)
		authRoute.Get("oauth/microsoft", oauth.MicrosoftGetSignInUrlHandler)
		authRoute.Post("oauth/microsoft", user.UserOauthLoginMicrosoftHandler)
		authRoute.Get("logout", user.UserLogoutHandler)
	}
	users := apiRouter.Group("users")
	{ // user
		users.Get("", user.MustAuthMiddleware, user.ListUsersHandler)
	}
	opportunities := apiRouter.Group("opportunities")
	{ // opportunity
		opportunities.Post("",
			user.MustAuthMiddleware,
			storage.UploadHandlerFactory(&storage.UploadConfig{
				NonFormCallNext: true,
				// jpeg, pdf, xlsx, docs, pptx
				AllowedTypes: []string{
					"image/jpeg",
					"image/png",
					"application/pdf",
					"application/msword",
					"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
					"application/vnd.ms-excel",
					"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
					"application/vnd.ms-powerpoint",
					"application/vnd.openxmlformats-officedocument.presentationml.presentation",
				},
				MaxSize:         100 * 1024 * 1024,
				PathPrefix:      "opportunities",
				PathIncludeHash: true,
			}),
			opportunity.CreateOpportunityHandler)
		opportunities.Get("", user.MustAuthMiddleware, opportunity.ListOpportunitiesHandler)
		opportunities.Get("/:opportunityID", user.MustAuthMiddleware, opportunity.MustOpportunityIDMiddleware, opportunity.OpportunityDetailHandler)
		opportunities.Patch("/:opportunityID", user.MustAuthMiddleware, opportunity.MustOpportunityIDMiddleware, opportunity.OpportunityEditHandlerHandler)
		opportunities.Post("/:opportunityID/categories", user.MustAuthMiddleware, opportunity.MustOpportunityIDMiddleware, opportunity.CreateOpportunityCategoryHandler)
		opportunities.Patch("/:opportunityID/categories/:opportunityCategoryID",
			user.MustAuthMiddleware,
			opportunity.MustOpportunityIDMiddleware,
			opportunity.MustOpportunityCategoryIDMiddleware,
			opportunity.ChangeOpportunityCategoryHandler)
		opportunities.Post("/:opportunityID/categories/:opportunityCategoryID/files",
			user.MustAuthMiddleware,
			opportunity.MustOpportunityIDMiddleware,
			opportunity.MustOpportunityCategoryIDMiddleware,
			opportunity.CreateOpportunityCategoryFileHandler)
	}
	levels := apiRouter.Group("levels")
	{ // levels
		levels.Get("", user.MustAuthMiddleware, level.ListLevelsHandler)
	}
	statuses := apiRouter.Group("statuses")
	{ // statuses
		statuses.Get("", user.MustAuthMiddleware, status.ListStatusHandler)
	}
	clients := apiRouter.Group("clients")
	{ // clients
		clients.Get("", user.MustAuthMiddleware, client.ListClientsHandler)
		clients.Post("",
			user.MustAuthMiddleware,
			storage.UploadHandlerFactory(&storage.UploadConfig{
				NonFormCallNext: true,
				AllowedTypes:    []string{"image"},
				MaxSize:         2 * 1024 * 1024,
				PathPrefix:      "clients",
				PathIncludeHash: true,
			}),
			client.CreateClientHandler,
		)
		clients.Patch("/:clientId", user.MustAuthMiddleware, client.MustClientIDMiddleware, client.ChangeClientHandler)
		clients.Get("/:clientId", user.MustAuthMiddleware, client.MustClientIDMiddleware, client.ClientDetailHandler)
	}
}
